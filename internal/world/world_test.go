package world

import (
	"errors"
	"testing"
)

const owner = "nostarou"

func walk(w *World, max int) {
	for i := 0; i < max; i++ {
		w.Step()
	}
}

func TestMoveWalksToTarget(t *testing.T) {
	w := NewDefault()
	if err := w.Move(owner, "nostarou", Pos{3, 6}); err != nil {
		t.Fatal(err)
	}
	a, _ := w.Actor("nostarou")
	if a.Pos != (Pos{7, 6}) || a.Target == nil {
		t.Fatalf("should not teleport: %+v", a)
	}
	w.Step()
	a, _ = w.Actor("nostarou")
	if a.Pos != (Pos{6, 6}) {
		t.Fatalf("one step should move one tile, got %+v", a.Pos)
	}
	walk(w, 10)
	a, _ = w.Actor("nostarou")
	if a.Pos != (Pos{3, 6}) || a.Target != nil {
		t.Fatalf("did not arrive: %+v", a)
	}
}

func TestMoveBounds(t *testing.T) {
	w := NewDefault()
	for _, p := range []Pos{{-1, 0}, {0, -1}, {RoomWidth, 0}, {0, RoomHeight}} {
		if err := w.Move(owner, "nostarou", p); !errors.Is(err, ErrOutOfBounds) {
			t.Errorf("%v: want ErrOutOfBounds, got %v", p, err)
		}
	}
	if err := w.Move(owner, "nostarou", Pos{RoomWidth - 1, RoomHeight - 1}); err != nil {
		t.Errorf("corner should be valid: %v", err)
	}
}

func TestMoveBlockedByFurniture(t *testing.T) {
	w := NewDefault()
	if err := w.Move(owner, "nostarou", Pos{14, 5}); !errors.Is(err, ErrBlocked) {
		t.Fatalf("want ErrBlocked, got %v", err)
	}
}

func TestPathAvoidsFurniture(t *testing.T) {
	w := NewDefault()
	w.Move(owner, "nostarou", Pos{15, 5}) // behind the PC at (14,5)
	for i := 0; i < 30; i++ {
		w.Step()
		a, _ := w.Actor("nostarou")
		if a.Pos == (Pos{14, 5}) {
			t.Fatal("walked through furniture")
		}
	}
	a, _ := w.Actor("nostarou")
	if a.Pos != (Pos{15, 5}) {
		t.Fatalf("did not arrive: %+v", a.Pos)
	}
}

func TestInteractWindowWalksThenUses(t *testing.T) {
	w := NewDefault()
	var got []Event
	w.SetHook(func(e Event) {
		if e.Type == "interact" {
			got = append(got, e)
		}
	})
	if err := w.Interact(owner, "nostarou", "window"); err != nil {
		t.Fatal(err)
	}
	a, _ := w.Actor("nostarou")
	if a.Using != "" || len(got) != 0 {
		t.Fatalf("must not use furniture before arriving: %+v", a)
	}
	walk(w, 20)
	a, _ = w.Actor("nostarou")
	if a.Pos != (Pos{7, 1}) || a.Using != "window" || a.State != StateTalking {
		t.Fatalf("unexpected actor after walking: %+v", a)
	}
	if len(got) != 1 || got[0].Furniture.ID != "window" || got[0].By != owner {
		t.Fatalf("want exactly one interact event for window by owner, got %+v", got)
	}
}

func TestInteractStates(t *testing.T) {
	cases := map[string]string{"window": StateTalking, "pc": StateWorking, "bed": StateAway}
	for id, st := range cases {
		w := NewDefault()
		if err := w.Interact(owner, "nostarou", id); err != nil {
			t.Fatal(err)
		}
		walk(w, 30)
		a, _ := w.Actor("nostarou")
		if a.State != st || a.Using != id {
			t.Errorf("%s: got %+v", id, a)
		}
	}
}

func TestInteractUnknownFurniture(t *testing.T) {
	w := NewDefault()
	if err := w.Interact(owner, "nostarou", "bookshelf"); !errors.Is(err, ErrNoFurniture) {
		t.Fatalf("want ErrNoFurniture, got %v", err)
	}
}

func TestPermissions(t *testing.T) {
	w := NewDefault()
	for _, by := range []string{"", "stranger"} {
		if err := w.Move(by, "nostarou", Pos{1, 1}); !errors.Is(err, ErrForbidden) {
			t.Errorf("move by %q: want ErrForbidden, got %v", by, err)
		}
		if err := w.Interact(by, "nostarou", "pc"); !errors.Is(err, ErrForbidden) {
			t.Errorf("interact by %q: want ErrForbidden, got %v", by, err)
		}
	}
	a, _ := w.Actor("nostarou")
	if a.Target != nil {
		t.Fatal("forbidden request changed state")
	}
	// invited guest may operate
	w.rooms["nostarou-room"].Invited = []string{"labomi"}
	if err := w.Interact("labomi", "nostarou", "pc"); err != nil {
		t.Fatalf("invited should be allowed: %v", err)
	}
	// anyone with an id may knock, anonymous may not
	if err := w.Knock("stranger", "nostarou-room", "hi"); err != nil {
		t.Fatalf("knock: %v", err)
	}
	if err := w.Knock("", "nostarou-room", "hi"); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("anonymous knock: want ErrBadRequest, got %v", err)
	}
}

func TestSubscribeReceivesEvents(t *testing.T) {
	w := NewDefault()
	ch, cancel := w.Subscribe()
	defer cancel()
	w.Move(owner, "nostarou", Pos{7, 5})
	ev := <-ch
	if ev.Type != "actor" || ev.Actor.ID != "nostarou" {
		t.Fatalf("unexpected event %+v", ev)
	}
}
