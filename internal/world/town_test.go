package world

import (
	"errors"
	"slices"
	"testing"
)

func zoneByID(r *Room, id string) *Zone {
	for _, z := range r.Zones {
		if z.ID == id {
			return z
		}
	}
	return nil
}

// BFS from each resident: every free tile of every zone in the town (both
// houses and the garden) is reachable, so the two houses are connected through
// the garden and no room is sealed off.
func TestTownEveryZoneReachable(t *testing.T) {
	w := NewDefault()
	r := house(w)
	if r.Width != TownWidth || r.Height != TownHeight || len(r.Houses) != 2 {
		t.Fatalf("town is %dx%d with %d houses", r.Width, r.Height, len(r.Houses))
	}
	want := []string{"labomi-ldk", "labomi-entrance", "labomi-hallway", "labomi-room", "labomi-guest",
		"labomi-toilet", "labomi-washroom", "labomi-bath", "garden"}
	for _, id := range want {
		if zoneByID(r, id) == nil {
			t.Errorf("missing zone %s", id)
		}
	}
	for _, who := range []string{"labomi", "nostarou"} {
		a, _ := w.Actor(who)
		for _, z := range r.Zones {
			free := 0
			for y := z.Rect.Y; y < z.Rect.Y+z.Rect.H; y++ {
				for x := z.Rect.X; x < z.Rect.X+z.Rect.W; x++ {
					if p := (Pos{x, y}); !r.blocked(p) {
						free++
						if _, ok := r.findPath(a.Pos, p); !ok {
							t.Errorf("from %s: zone %s tile %v unreachable", who, z.ID, p)
						}
					}
				}
			}
			if free == 0 {
				t.Errorf("zone %s has no free tile", z.ID)
			}
		}
	}
}

// Zones never overlap or sit on walls, every zone lies inside its own house
// (or, for the garden, outside both), and furniture stays on the floor of one
// house without crossing into the neighbour's.
func TestTownLayoutSound(t *testing.T) {
	r := house(NewDefault())
	for y := 0; y < r.Height; y++ {
		for x := 0; x < r.Width; x++ {
			p, n := Pos{x, y}, 0
			for _, z := range r.Zones {
				if z.Rect.Contains(p) {
					n++
					if r.IsWall(p) {
						t.Errorf("zone %s on a wall at %v", z.ID, p)
					}
				}
			}
			if n > 1 {
				t.Errorf("zones overlap at %v", p)
			}
		}
	}
	for _, z := range r.Zones {
		corners := []Pos{{z.Rect.X, z.Rect.Y}, {z.Rect.X + z.Rect.W - 1, z.Rect.Y + z.Rect.H - 1}}
		for _, c := range corners {
			h := r.HouseAt(c)
			if (h == nil && z.House != "") || (h != nil && h.ID != z.House) {
				t.Errorf("zone %s (house %q) has corner %v in house %v", z.ID, z.House, c, h)
			}
		}
	}
	for _, f := range r.Furniture {
		h0 := r.HouseAt(f.Pos)
		for y := f.Pos.Y; y < f.Pos.Y+max(f.Size.H, 1); y++ {
			for x := f.Pos.X; x < f.Pos.X+max(f.Size.W, 1); x++ {
				if r.HouseAt(Pos{x, y}) != h0 {
					t.Errorf("%s straddles two houses at (%d,%d)", f.ID, x, y)
				}
			}
		}
	}
}

// The stepping stones join labomi's front door to nostarou's across the garden,
// and walking between the houses never steps on a wall.
func TestGardenJoinsFrontDoors(t *testing.T) {
	w := NewDefault()
	r := house(w)
	for x := GardenX; x < GardenX+GardenW; x++ {
		if r.blocked(Pos{x, 4}) {
			t.Errorf("stepping stone %d blocked", x)
		}
	}
	if !r.IsDoor(Pos{LabomiW - 1, 4}) || !r.IsDoor(Pos{NostarouX, 4}) {
		t.Fatal("front doors must face each other across the stones")
	}
	w.House(LabomiHouse).Invited = []string{"nostarou"}
	if err := w.Move("nostarou", "nostarou", Pos{11, 4}); err != nil { // labomi's LDK
		t.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		w.Step()
		a, _ := w.Actor("nostarou")
		if r.IsWall(a.Pos) {
			t.Fatalf("walked through a wall at %v", a.Pos)
		}
	}
	if a, _ := w.Actor("nostarou"); a.Pos != (Pos{11, 4}) {
		t.Fatalf("did not arrive at labomi's: %v", a.Pos)
	}
}

// Regression (per-house visibility): owning one house must give nothing in the
// other. Before zones were tied to a House, visibility was judged against the
// room's single owner, so nostarou (owner of the room) saw labomi's private
// rooms. Invited ids see the host's invited zones only.
func TestPerHouseVisibility(t *testing.T) {
	r := house(NewDefault())
	r.House(NostarouHouse).Invited = []string{"labomi"}
	r.House(LabomiHouse).Invited = []string{"nostarou"}
	cases := []struct {
		zone, viewer string
		want         bool
	}{
		// someone else's owner-only zones: hidden even from a guest / neighbour
		{"labomi-room", "nostarou", false}, {"labomi-bath", "nostarou", false}, {"labomi-washroom", "nostarou", false},
		{"bedroom", "labomi", false}, {"study", "labomi", false}, {"bath", "labomi", false},
		// invited zones: visible to the host's invited guest
		{"labomi-guest", "nostarou", true}, {"hobby", "labomi", true}, {"guest", "labomi", true},
		// each owner sees its own house
		{"labomi-room", "labomi", true}, {"labomi-guest", "labomi", true}, {"bedroom", "nostarou", true},
		// strangers / anonymous: public only
		{"labomi-guest", "stranger", false}, {"labomi-room", "", false}, {"labomi-ldk", "", true}, {"garden", "", true},
	}
	for _, c := range cases {
		if got := r.CanSeeZone(c.viewer, zoneByID(r, c.zone)); got != c.want {
			t.Errorf("%s by %q: got %v want %v", c.zone, c.viewer, got, c.want)
		}
	}
	// uninvited: labomi's invited room is hidden from nostarou
	r.House(LabomiHouse).Invited = nil
	if r.CanSeeZone("nostarou", zoneByID(r, "labomi-guest")) {
		t.Error("not invited: labomi-guest must be hidden from nostarou")
	}
}

// Same rule through the snapshot: nostarou standing in labomi's room is
// redacted for everyone but labomi (and itself would be, too: it is a guest).
func TestSnapshotHidesActorInNeighboursPrivateRoom(t *testing.T) {
	w := NewDefault()
	w.actors["labomi"].Pos = Pos{4, 14} // labomi's own room
	for _, viewer := range []string{"nostarou", "", "stranger"} {
		s := w.ViewSnapshot(viewer)
		if a := actorIn(s, "labomi"); !a.Hidden || a.Pos != (Pos{}) {
			t.Errorf("%q sees labomi in her room: %+v", viewer, a)
		}
		if !slices.Contains(s.Rooms[0].HiddenZones, "labomi-room") {
			t.Errorf("%q: labomi-room must be hidden", viewer)
		}
	}
	if a := actorIn(w.ViewSnapshot("labomi"), "labomi"); a.Hidden {
		t.Error("labomi must see herself in her room")
	}
}

// Operating rights per house: an uninvited neighbour may not walk into the
// other house nor order its owner around; once invited it may visit, and the
// host may guide a guest standing in the host's house.
func TestPerHousePermissions(t *testing.T) {
	w := NewDefault()
	if err := w.Move("nostarou", "nostarou", Pos{11, 4}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("uninvited visit: want ErrForbidden, got %v", err)
	}
	if err := w.Move("nostarou", "labomi", Pos{5, 7}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("moving the neighbour: want ErrForbidden, got %v", err)
	}
	if err := w.Interact("nostarou", "nostarou", "labomi-sofa"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("using the neighbour's sofa uninvited: want ErrForbidden, got %v", err)
	}
	if err := w.Move("nostarou", "nostarou", Pos{23, 4}); err != nil { // garden is open
		t.Fatalf("garden: %v", err)
	}
	if err := w.SetInvited("labomi", []string{"nostarou"}); err != nil { // by owner id
		t.Fatal(err)
	}
	if err := w.Interact("nostarou", "nostarou", "labomi-sofa"); err != nil {
		t.Fatalf("invited visit: %v", err)
	}
	walk(w, 200)
	if a, _ := w.Actor("nostarou"); a.Using != "labomi-sofa" {
		t.Fatalf("did not sit on labomi's sofa: %+v", a)
	}
	if err := w.Move("labomi", "nostarou", Pos{12, 7}); err != nil { // host guides the guest
		t.Fatalf("host guiding guest: %v", err)
	}
	if err := w.SetInvited("nowhere", nil); !errors.Is(err, ErrNoHouse) {
		t.Fatalf("unknown house: %v", err)
	}
}
