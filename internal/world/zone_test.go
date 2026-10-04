package world

import (
	"errors"
	"testing"
)

func house(w *World) *Room { return w.rooms["nostarou-room"] }

// Layout sanity: furniture never sits on a wall (except the window, which is set
// into the outer wall), every usable piece can be reached from the start tile.
func TestLayoutConsistent(t *testing.T) {
	w := NewDefault()
	r := house(w)
	a, _ := w.Actor("nostarou")
	for _, f := range r.Furniture {
		for y := f.Pos.Y; y < f.Pos.Y+max(f.Size.H, 1); y++ {
			for x := f.Pos.X; x < f.Pos.X+max(f.Size.W, 1); x++ {
				if f.Kind != KindWindow && f.Kind != KindClock && (r.IsWall(Pos{x, y}) || r.ZoneAt(Pos{x, y}) == nil) {
					t.Errorf("%s covers non-floor tile (%d,%d)", f.ID, x, y)
				}
			}
		}
		if f.Function == "" {
			continue
		}
		if r.blocked(f.Access) {
			t.Errorf("%s: access tile %v is blocked", f.ID, f.Access)
		}
		if _, ok := r.findPath(a.Pos, f.Access); !ok {
			t.Errorf("%s: access tile %v unreachable", f.ID, f.Access)
		}
	}
	for _, z := range r.Zones {
		c := Pos{z.Rect.X, z.Rect.Y + z.Rect.H - 1}
		for r.blocked(c) {
			c.X++
		}
		if _, ok := r.findPath(a.Pos, c); !ok {
			t.Errorf("zone %s unreachable", z.ID)
		}
	}
}

func TestWallsBlockAndDoorsPass(t *testing.T) {
	w := NewDefault()
	if err := w.Move(owner, "nostarou", Pos{8, 15}); !errors.Is(err, ErrBlocked) {
		t.Fatalf("wall tile: want ErrBlocked, got %v", err)
	}
	if err := w.Move(owner, "nostarou", Pos{4, 12}); err != nil {
		t.Fatalf("door tile should be walkable: %v", err)
	}
}

// Bedroom -> study are neighbours separated by the wall at x=8. The path must go
// out through the bedroom door, along the hallway and in through the study door,
// never stepping on a wall tile.
func TestPathGoesAroundWalls(t *testing.T) {
	w := NewDefault()
	r := house(w)
	w.actors["nostarou"].Pos = Pos{7, 16}
	if err := w.Move(owner, "nostarou", Pos{9, 16}); err != nil {
		t.Fatal(err)
	}
	steps, door := 0, map[Pos]bool{}
	for i := 0; i < 100; i++ {
		w.Step()
		a, _ := w.Actor("nostarou")
		if r.IsWall(a.Pos) {
			t.Fatalf("walked through a wall at %v", a.Pos)
		}
		if r.IsDoor(a.Pos) {
			door[a.Pos] = true
		}
		steps++
		if a.Target == nil {
			break
		}
	}
	a, _ := w.Actor("nostarou")
	if a.Pos != (Pos{9, 16}) {
		t.Fatalf("did not arrive: %v", a.Pos)
	}
	if !door[Pos{4, 12}] || !door[Pos{12, 12}] || steps <= 2 {
		t.Fatalf("expected detour through both doors, steps=%d doors=%v", steps, door)
	}
}

func TestZoneVisibilityRules(t *testing.T) {
	r := house(NewDefault())
	r.Invited = []string{"labomi"}
	cases := []struct {
		zone, viewer string
		want         bool
	}{
		{"living", "", true}, {"entrance", "stranger", true}, {"kitchen", "", true},
		{"bedroom", "", false}, {"bedroom", "labomi", false}, {"bedroom", "nostarou", true},
		{"study", "stranger", false}, {"study", "nostarou", true},
		{"toilet", "", false}, {"toilet", "nostarou", true}, {"bath", "labomi", false}, {"washroom", "nostarou", true},
		{"hobby", "", false}, {"hobby", "labomi", true}, {"guest", "stranger", false}, {"guest", "labomi", true},
	}
	for _, c := range cases {
		var z *Zone
		for _, zz := range r.Zones {
			if zz.ID == c.zone {
				z = zz
			}
		}
		if got := r.CanSeeZone(c.viewer, z); got != c.want {
			t.Errorf("%s by %q: got %v want %v", c.zone, c.viewer, got, c.want)
		}
	}
	// bedroom doorway is not a zone but must not leak either
	if r.CanSeeTile("", Pos{4, 12}) || !r.CanSeeTile("nostarou", Pos{4, 12}) {
		t.Error("bedroom doorway visibility wrong")
	}
	if !r.CanSeeTile("", Pos{23, 9}) {
		t.Error("LDK doorway should be public")
	}
	if r.CanSeeZone("nostarou", &Zone{Visibility: "bogus"}) != true || r.CanSeeZone("x", &Zone{Visibility: "bogus"}) {
		t.Error("unknown visibility must fail closed (owner only)")
	}
}

func TestViewSnapshotHidesActorInBedroom(t *testing.T) {
	w := NewDefault()
	w.actors["nostarou"].Pos = Pos{3, 15}
	w.actors["nostarou"].State = StateAway
	w.actors["nostarou"].Using = "bed"

	pub := w.ViewSnapshot("")
	a := pub.Actors[0]
	if !a.Hidden || a.Pos != (Pos{}) || a.State != StateHidden || a.Using != "" {
		t.Fatalf("public viewer sees private actor: %+v", a)
	}
	if hz := pub.Rooms[0].HiddenZones; len(hz) != 7 {
		t.Fatalf("anonymous should have 7 hidden zones, got %v", hz)
	}
	own := w.ViewSnapshot("nostarou")
	if a := own.Actors[0]; a.Hidden || a.Pos != (Pos{3, 15}) || a.Using != "bed" {
		t.Fatalf("owner must see everything: %+v", a)
	}
	if len(own.Rooms[0].HiddenZones) != 0 {
		t.Fatal("owner has no hidden zones")
	}
	// the world itself is untouched by filtering
	if got, _ := w.Actor("nostarou"); got.Pos != (Pos{3, 15}) {
		t.Fatalf("filter mutated world: %+v", got)
	}
}

func TestFilterEvent(t *testing.T) {
	w := NewDefault()
	bed := *house(w).furniture("bed")
	inBed := &Actor{ID: "nostarou", Name: "のすたろう", RoomID: "nostarou-room", Pos: Pos{3, 15}, State: StateAway, Using: "bed"}

	if _, ok := w.FilterEvent("", Event{Type: "interact", Actor: inBed, Furniture: &bed, By: owner}); ok {
		t.Fatal("interact in bedroom must be dropped for the public")
	}
	ev, ok := w.FilterEvent("", Event{Type: "actor", Actor: inBed, By: owner})
	if !ok || !ev.Actor.Hidden || ev.Actor.Pos != (Pos{}) || ev.By != "" {
		t.Fatalf("actor event must be redacted: %+v %+v", ev, ev.Actor)
	}
	if ev, ok := w.FilterEvent(owner, Event{Type: "interact", Actor: inBed, Furniture: &bed}); !ok || ev.Actor.Hidden {
		t.Fatal("owner must receive bedroom events")
	}
	// walking in public LDK toward the bedroom: position visible, target withheld
	tgt := Pos{3, 15}
	walking := &Actor{ID: "nostarou", RoomID: "nostarou-room", Pos: Pos{22, 3}, Target: &tgt}
	ev, ok = w.FilterEvent("", Event{Type: "actor", Actor: walking})
	if !ok || ev.Actor.Hidden || ev.Actor.Target != nil || ev.Actor.Pos != (Pos{22, 3}) {
		t.Fatalf("public walk: %+v", ev.Actor)
	}
	if walking.Target == nil {
		t.Fatal("filter mutated the original event")
	}
	// knock has no actor: passes through
	if _, ok := w.FilterEvent("", Event{Type: "knock", By: "labomi"}); !ok {
		t.Fatal("knock should pass")
	}
}
