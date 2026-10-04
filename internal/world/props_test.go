package world

import (
	"slices"
	"testing"
)

// Props must not cut the house apart: every free floor tile of every zone is
// reachable from the start tile, no furniture sits on a door, and every door has
// a free floor tile on both sides.
func TestPropsKeepEverythingReachable(t *testing.T) {
	w := NewDefault()
	r := house(w)
	a, _ := w.Actor("nostarou")
	for _, z := range r.Zones {
		free := 0
		for y := z.Rect.Y; y < z.Rect.Y+z.Rect.H; y++ {
			for x := z.Rect.X; x < z.Rect.X+z.Rect.W; x++ {
				p := Pos{x, y}
				if r.blocked(p) {
					continue
				}
				free++
				if _, ok := r.findPath(a.Pos, p); !ok {
					t.Errorf("zone %s: tile %v unreachable", z.ID, p)
				}
			}
		}
		if free == 0 {
			t.Errorf("zone %s has no free tile", z.ID)
		}
	}
	for _, d := range r.Doors {
		if r.blocked(d) {
			t.Errorf("door %v is blocked", d)
		}
		if d.X == 0 { // front door: outside is off the map
			continue
		}
		sides := 0
		for _, n := range []Pos{{d.X + 1, d.Y}, {d.X - 1, d.Y}, {d.X, d.Y + 1}, {d.X, d.Y - 1}} {
			if r.inBounds(n) && !r.blocked(n) && r.ZoneAt(n) != nil {
				sides++
			}
		}
		if sides < 2 {
			t.Errorf("door %v: furniture blocks one side (%d free sides)", d, sides)
		}
	}
	// each room got its small things; ids are unique
	seen := map[string]bool{}
	for _, f := range r.Furniture {
		if seen[f.ID] {
			t.Errorf("duplicate furniture id %s", f.ID)
		}
		seen[f.ID] = true
	}
	for _, id := range []string{"toiletbowl", "paper", "vanity", "washer", "bathtub", "shower", "deskchair", "bedrug", "clock"} {
		if !seen[id] {
			t.Errorf("missing %s", id)
		}
	}
}

// Flat props (rugs, desk chair) are walkable; solid ones block.
func TestWalkableProps(t *testing.T) {
	r := house(NewDefault())
	if r.blocked(Pos{13, 14}) { // desk chair = PC access tile
		t.Error("desk chair must be walkable")
	}
	if !r.blocked(Pos{7, 6}) { // toilet bowl
		t.Error("toilet bowl must block")
	}
}

func step(w *World, n int) {
	for i := 0; i < n; i++ {
		w.Step()
	}
}

// In-use privacy: while someone is in the toilet / bath, only that someone sees
// inside -- not the owner, not invited, not the public. Only the door flag leaks.
func TestPrivateZoneInUse(t *testing.T) {
	w := NewDefault()
	r := house(w)
	r.Invited = []string{"labomi"}
	w.AddActor(&Actor{ID: "labomi", Name: "らぼみ", RoomID: r.ID, Pos: Pos{22, 3}})

	// empty: normal visibility (owner sees the bath zone)
	if s := w.ViewSnapshot(owner); slices.Contains(s.Rooms[0].HiddenZones, "bath") || len(s.Rooms[0].InUse) != 0 {
		t.Fatalf("empty bath: owner view %+v", s.Rooms[0].HiddenZones)
	}

	events, cancel := w.Subscribe()
	defer cancel()
	if err := w.Move("labomi", "labomi", Pos{17, 7}); err != nil { // into the bath
		t.Fatal(err)
	}
	step(w, 60)
	if a, _ := w.Actor("labomi"); a.Pos != (Pos{17, 7}) {
		t.Fatalf("labomi did not reach the bath: %v", a.Pos)
	}

	for _, viewer := range []string{owner, "", "stranger"} {
		s := w.ViewSnapshot(viewer)
		if !slices.Contains(s.Rooms[0].HiddenZones, "bath") || !slices.Equal(s.Rooms[0].InUse, []string{"bath"}) {
			t.Errorf("%q: bath must be hidden and flagged in use: hidden=%v in_use=%v", viewer, s.Rooms[0].HiddenZones, s.Rooms[0].InUse)
		}
		for _, a := range s.Actors {
			if a.ID == "labomi" && (!a.Hidden || a.Pos != (Pos{})) {
				t.Errorf("%q sees labomi in the bath: %+v", viewer, a)
			}
		}
	}
	self := w.ViewSnapshot("labomi")
	if slices.Contains(self.Rooms[0].HiddenZones, "bath") {
		t.Error("the one inside must see the bath")
	}
	for _, a := range self.Actors {
		if a.ID == "labomi" && (a.Hidden || a.Pos != (Pos{17, 7})) {
			t.Errorf("labomi cannot see herself: %+v", a)
		}
	}

	// the stream: the owner never receives labomi's bath position; an occupancy
	// event announces only the in-use flag.
	sawOcc := false
	for len(events) > 0 {
		ev := <-events
		got, ok := w.FilterEvent(owner, ev)
		if !ok {
			continue
		}
		if got.Type == EventOccupancy {
			sawOcc = sawOcc || slices.Equal(got.InUse, []string{"bath"})
			if got.Actor != nil || got.By != "" {
				t.Errorf("occupancy event leaks a person: %+v", got)
			}
		}
		if got.Actor != nil && got.Actor.ID == "labomi" && !got.Actor.Hidden && r.ZoneAt(got.Actor.Pos) != nil && r.ZoneAt(got.Actor.Pos).ID == "bath" {
			t.Errorf("owner stream leaked labomi in the bath: %+v", got.Actor)
		}
	}
	if !sawOcc {
		t.Error("no occupancy event for the bath")
	}

	// leave: back to normal visibility
	if err := w.Move("labomi", "labomi", Pos{22, 3}); err != nil {
		t.Fatal(err)
	}
	step(w, 60)
	if s := w.ViewSnapshot(owner); slices.Contains(s.Rooms[0].HiddenZones, "bath") || len(s.Rooms[0].InUse) != 0 {
		t.Fatalf("after leaving: hidden=%v in_use=%v", s.Rooms[0].HiddenZones, s.Rooms[0].InUse)
	}
}
