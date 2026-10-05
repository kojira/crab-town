package world

import (
	"errors"
	"testing"
)

const visitor = "nostr:0123456789abcdef"

func joinVisitor(t *testing.T, actsAs string) *World {
	t.Helper()
	w := NewDefault()
	ok, err := w.Join(Actor{ID: visitor, Name: "オーナー", Pubkey: "0123456789abcdef0123", RoomID: TownID, Pos: Pos{GardenX + 2, 5}}, actsAs)
	if !ok || err != nil {
		t.Fatalf("join: %v %v", ok, err)
	}
	return w
}

// The visitor walks with nostarou's house rights: the garden, nostarou's house
// (owner zones included) -- but not labomi's house uninvited.
func TestVisitorWalksWithActsAsRights(t *testing.T) {
	w := joinVisitor(t, "nostarou")
	for _, p := range []Pos{{24, 4}, {NostarouX + 3, 15}} {
		if err := w.Move(visitor, visitor, p); err != nil {
			t.Fatalf("move to %v: %v", p, err)
		}
	}
	if err := w.Move(visitor, visitor, Pos{11, 4}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("uninvited labomi house: %v", err)
	}
	w.SetInvited(LabomiHouse, []string{"nostarou"})
	if err := w.Move(visitor, visitor, Pos{11, 4}); err != nil {
		t.Fatalf("invited labomi house: %v", err)
	}
}

// Without actsAs the visitor is just a guest: garden only.
func TestVisitorWithoutRightsStaysInGarden(t *testing.T) {
	w := joinVisitor(t, "")
	if err := w.Move(visitor, visitor, Pos{24, 4}); err != nil {
		t.Fatalf("garden: %v", err)
	}
	if err := w.Move(visitor, visitor, Pos{NostarouX + 3, 15}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("house without rights: %v", err)
	}
}

// actsAs lends house rights only: the visitor cannot drive nostarou, and
// nostarou (the host) cannot drive the visitor standing in its house.
func TestVisitorAndResidentDoNotDriveEachOther(t *testing.T) {
	w := joinVisitor(t, "nostarou")
	if err := w.Move(visitor, "nostarou", Pos{24, 4}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("visitor drove nostarou: %v", err)
	}
	if err := w.Move(visitor, visitor, Pos{NostarouX + 2, 4}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		w.Step()
	}
	if err := w.Move("nostarou", visitor, Pos{NostarouX + 3, 15}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("nostarou drove the visitor: %v", err)
	}
}

// Visibility is unchanged: in an owner-only room the visitor is hidden from the public.
func TestVisitorFollowsVisibility(t *testing.T) {
	w := joinVisitor(t, "nostarou")
	if a := findActor(w.ViewSnapshot(""), visitor); a == nil || a.Hidden || a.Pubkey == "" {
		t.Fatalf("visitor in the garden: %+v", a)
	}
	w.Move(visitor, visitor, Pos{NostarouX + 3, 15})
	for i := 0; i < 60; i++ {
		w.Step()
	}
	if a := findActor(w.ViewSnapshot(""), visitor); a == nil || !a.Hidden || a.Pubkey != "" {
		t.Fatalf("visitor in the bedroom leaks to the public: %+v", a)
	}
	if again, _ := w.Join(Actor{ID: visitor, RoomID: TownID, Pos: Pos{24, 4}}, ""); again {
		t.Fatal("join twice")
	}
}

func findActor(s Snapshot, id string) *Actor {
	for _, a := range s.Actors {
		if a.ID == id {
			return a
		}
	}
	return nil
}
