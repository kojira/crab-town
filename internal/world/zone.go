package world

import "sort"

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

// CanSeeZone reports whether viewer may see what happens inside z.
// Unknown visibility values are treated as owner-only (fail closed).
func (r *Room) CanSeeZone(viewer string, z *Zone) bool {
	switch z.Visibility {
	case VisPublic:
		return true
	case VisInvited:
		return r.CanOperate(viewer)
	}
	return viewer != "" && viewer == r.Owner
}

// CanSeeTile reports whether viewer may see an actor standing on p.
// A tile outside every zone (a door) is visible only if every zone touching it
// is visible, so standing in a bedroom doorway does not leak to the public.
func (r *Room) CanSeeTile(viewer string, p Pos) bool {
	if z := r.ZoneAt(p); z != nil {
		return r.CanSeeZone(viewer, z)
	}
	for _, d := range []Pos{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		if z := r.ZoneAt(Pos{p.X + d.X, p.Y + d.Y}); z != nil && !r.CanSeeZone(viewer, z) {
			return false
		}
	}
	return true
}

// hiddenZones lists the ids of zones viewer may not see.
func (r *Room) hiddenZones(viewer string) []string {
	var out []string
	for _, z := range r.Zones {
		if !r.CanSeeZone(viewer, z) {
			out = append(out, z.ID)
		}
	}
	return out
}

// redactActor returns the actor as viewer may see it. Position, state, furniture
// and target inside invisible zones are withheld; only id / name / room remain.
func (r *Room) redactActor(viewer string, a *Actor) *Actor {
	c := copyActor(a)
	if !r.CanSeeTile(viewer, a.Pos) {
		return &Actor{ID: a.ID, Name: a.Name, RoomID: a.RoomID, State: StateHidden, Hidden: true}
	}
	if c.Target != nil && !r.CanSeeTile(viewer, *c.Target) {
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
	for _, r := range s.Rooms {
		if room := w.rooms[r.ID]; room != nil {
			r.HiddenZones = room.hiddenZones(viewer)
		}
	}
	for i, a := range s.Actors {
		if room := w.rooms[a.RoomID]; room != nil {
			s.Actors[i] = room.redactActor(viewer, a)
		}
	}
	sort.Slice(s.Actors, func(i, j int) bool { return s.Actors[i].ID < s.Actors[j].ID })
	return s
}

// FilterEvent returns ev as viewer may see it; ok=false means drop it.
// Interact events inside invisible zones are dropped (the furniture would leak
// what the actor is doing); actor events are redacted.
func (w *World) FilterEvent(viewer string, ev Event) (Event, bool) {
	if ev.Actor == nil {
		return ev, true
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	room := w.rooms[ev.Actor.RoomID]
	if room == nil {
		return ev, false // fail closed
	}
	if !room.CanSeeTile(viewer, ev.Actor.Pos) {
		if ev.Type != "actor" {
			return Event{}, false
		}
		ev.By = ""
	}
	ev.Actor = room.redactActor(viewer, ev.Actor)
	return ev, true
}
