package world

import (
	"slices"
	"sort"
)

// Size is a furniture footprint in tiles.
type Size struct {
	W int `json:"w"`
	H int `json:"h"`
}

// Rect is a tile rectangle (X, Y = top-left).
type Rect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

func (r Rect) Contains(p Pos) bool {
	return p.X >= r.X && p.Y >= r.Y && p.X < r.X+r.W && p.Y < r.Y+r.H
}

// Zone is a walled area inside a room (bedroom, living, ...).
type Zone struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Floor      string `json:"floor"`      // floor texture hint for the viewer
	Visibility string `json:"visibility"` // public / invited / owner
	Rect       Rect   `json:"rect"`
	House      string `json:"house,omitempty"` // owning house id; "" = outdoors (garden)
	// Private: in-use privacy (toilet, bath). While any actor is inside, only the
	// actors inside may see it -- this overrides Visibility, the owner included.
	Private bool `json:"private,omitempty"`
}

// occupancy maps a private zone id to the actors inside it (in-use zones only).
type occupancy map[string][]string

// inUse lists the occupied private zones (sorted). Only this leaves the house.
func (o occupancy) inUse() []string {
	out := []string{}
	for id := range o {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// IsDoor reports whether p is a door (a passable gap in a wall).
func (r *Room) IsDoor(p Pos) bool {
	for _, d := range r.Doors {
		if d == p {
			return true
		}
	}
	return false
}

// IsWall reports whether p is an impassable wall tile (doors are not walls).
func (r *Room) IsWall(p Pos) bool {
	if r.IsDoor(p) {
		return false
	}
	for _, w := range r.Walls {
		if w.Contains(p) {
			return true
		}
	}
	return false
}

// ZoneAt returns the zone containing p, or nil (walls, doors).
func (r *Room) ZoneAt(p Pos) *Zone {
	for _, z := range r.Zones {
		if z.Rect.Contains(p) {
			return z
		}
	}
	return nil
}

// CanSeeZone reports whether viewer may see what happens inside z (ignoring
// occupancy). invited / owner are judged against the house the zone belongs
// to, not the room: owning one house gives nothing in the neighbour's.
// Unknown visibility values are treated as owner-only (fail closed).
func (r *Room) CanSeeZone(viewer string, z *Zone) bool { return r.canSeeZone(viewer, z, nil) }

func (r *Room) canSeeZone(viewer string, z *Zone, occ occupancy) bool {
	if in := occ[z.ID]; len(in) > 0 {
		return viewer != "" && slices.Contains(in, viewer)
	}
	if z.Visibility == VisPublic {
		return true
	}
	h := r.zoneHouse(z)
	if h == nil {
		return false // a non-public zone outside any house: nobody (fail closed)
	}
	if z.Visibility == VisInvited {
		return h.CanOperate(viewer)
	}
	return viewer != "" && viewer == h.Owner
}

// CanSeeTile reports whether viewer may see an actor standing on p.
// A tile outside every zone (a door) is visible only if every zone touching it
// is visible, so standing in a bedroom doorway does not leak to the public.
func (r *Room) CanSeeTile(viewer string, p Pos) bool { return r.canSeeTile(viewer, p, nil) }

func (r *Room) canSeeTile(viewer string, p Pos, occ occupancy) bool {
	if z := r.ZoneAt(p); z != nil {
		return r.canSeeZone(viewer, z, occ)
	}
	for _, d := range []Pos{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		if z := r.ZoneAt(Pos{p.X + d.X, p.Y + d.Y}); z != nil && !r.canSeeZone(viewer, z, occ) {
			return false
		}
	}
	return true
}

// hiddenZones lists the ids of zones viewer may not see.
func (r *Room) hiddenZones(viewer string, occ occupancy) []string {
	out := []string{}
	for _, z := range r.Zones {
		if !r.canSeeZone(viewer, z, occ) {
			out = append(out, z.ID)
		}
	}
	return out
}

// redactActor returns the actor as viewer may see it. Position, state, furniture
// and target inside invisible zones are withheld; only id / name / room remain.
func (r *Room) redactActor(viewer string, a *Actor, occ occupancy) *Actor {
	c := copyActor(a)
	if !r.canSeeTile(viewer, a.Pos, occ) {
		return &Actor{ID: a.ID, Name: a.Name, RoomID: a.RoomID, State: StateHidden, Hidden: true}
	}
	if c.Target != nil && !r.canSeeTile(viewer, *c.Target, occ) {
		c.Target = nil // do not reveal where in the private area it is heading
	}
	return c
}

// ViewSnapshot is Snapshot filtered for viewer ("" = anonymous).
func (w *World) ViewSnapshot(viewer string) Snapshot {
	s := w.Snapshot()
	w.mu.Lock()
	defer w.mu.Unlock()
	s.Viewer = viewer
	occ := map[string]occupancy{}
	for _, r := range s.Rooms {
		if room := w.rooms[r.ID]; room != nil {
			occ[r.ID] = w.occupancyLocked(room)
			r.HiddenZones = room.hiddenZones(viewer, occ[r.ID])
			r.InUse = occ[r.ID].inUse()
		}
	}
	for i, a := range s.Actors {
		if room := w.rooms[a.RoomID]; room != nil {
			s.Actors[i] = room.redactActor(viewer, a, occ[a.RoomID])
		}
	}
	sort.Slice(s.Actors, func(i, j int) bool { return s.Actors[i].ID < s.Actors[j].ID })
	return s
}

// FilterEvent returns ev as viewer may see it; ok=false means drop it.
// Interact events inside invisible zones are dropped (the furniture would leak
// what the actor is doing); actor events are redacted.
func (w *World) FilterEvent(viewer string, ev Event) (Event, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if ev.Type == EventOccupancy {
		room := w.rooms[ev.Room]
		if room == nil {
			return ev, false
		}
		// recompute for this viewer: the door flags plus which zones to frost
		occ := w.occupancyLocked(room)
		ev.InUse, ev.HiddenZones = occ.inUse(), room.hiddenZones(viewer, occ)
		return ev, true
	}
	if ev.Actor == nil {
		return ev, true
	}
	room := w.rooms[ev.Actor.RoomID]
	if room == nil {
		return ev, false // fail closed
	}
	occ := w.occupancyLocked(room)
	occ.add(room, ev.Actor) // the event's own position counts even if it has moved on
	if !room.canSeeTile(viewer, ev.Actor.Pos, occ) {
		if ev.Type != "actor" {
			return Event{}, false
		}
		ev.By = ""
	}
	ev.Actor = room.redactActor(viewer, ev.Actor, occ)
	return ev, true
}
