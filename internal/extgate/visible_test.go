package extgate

import (
	"strings"
	"testing"

	"github.com/kojira/crab-town/internal/world"
)

// coreRenderLimit: opencrab core shows an external said to the model cut to
// this many characters, so what the agent must know has to come before it.
const coreRenderLimit = 2000

const ownerID = "nostr:b3e43e8cc7e6dff2"

func joinOwner(t *testing.T, w *world.World, p world.Pos) {
	t.Helper()
	ok, err := w.Join(world.Actor{ID: ownerID, Name: "オーナー", Role: world.RoleOwner, Pubkey: "b3e43e8cc7e6dff2", RoomID: world.TownID, Pos: p}, "nostarou")
	if !ok || err != nil {
		t.Fatalf("join: %v %v", ok, err)
	}
}

func head(s string) string {
	if r := []rune(s); len(r) > coreRenderLimit {
		return string(r[:coreRenderLimit])
	}
	return s
}

func lookText(t *testing.T, b *Bridge) string {
	t.Helper()
	for _, op := range b.Operations() {
		if op.Name == "look" {
			res, err := op.Run(map[string]any{})
			if err != nil {
				t.Fatal(err)
			}
			return res.(map[string]any)["map"].(string)
		}
	}
	t.Fatal("no look operation")
	return ""
}

// The owner standing next to nostarou in the living room (the reported case:
// owner (47,3), nostarou (48,3)) is in every said -- talk, knock, tick -- and
// in look, with name, kind and position, within what core passes on.
func TestOwnerInSameRoomIsInSaidAndLook(t *testing.T) {
	w := world.NewDefault()
	b := &Bridge{World: w, Actor: "nostarou"}
	joinOwner(t, w, world.Pos{X: 47, Y: 3})
	want := "オーナー[オーナー](47,3)"

	talk, ok := b.toSaid(world.Event{Type: world.EventTalk, By: ownerID, To: "nostarou", Role: "owner", Message: "今目の前にいるよ"})
	if !ok {
		t.Fatal("talk not said")
	}
	knock, ok := b.toSaid(world.Event{Type: "knock", By: ownerID, House: world.NostarouHouse, Message: "こんにちは"})
	if !ok {
		t.Fatal("knock not said")
	}
	for name, text := range map[string]string{"talk": talk.Text, "knock": knock.Text, "tick": b.tickSaid().Text, "look": lookText(t, b)} {
		if !strings.Contains(head(text), want) {
			t.Errorf("%s: %q not within the first %d chars:\n%s", name, want, coreRenderLimit, head(text))
		}
		if !strings.Contains(head(text), "らぼみ[住人](8,6)") {
			t.Errorf("%s: resident labomi missing", name)
		}
	}
}

// An actor in a zone nostarou may not see (labomi's bedroom) is not listed,
// neither the owner avatar nor a resident.
func TestActorInInvisibleZoneIsNotListed(t *testing.T) {
	w := world.NewDefault()
	b := &Bridge{World: w, Actor: "nostarou"}
	joinOwner(t, w, world.Pos{X: 4, Y: 16}) // labomi's bedroom
	if err := w.Move("labomi", "labomi", world.Pos{X: 3, Y: 16}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		w.Step()
	}
	for name, text := range map[string]string{"tick": b.tickSaid().Text, "look": lookText(t, b)} {
		for _, leak := range []string{"オーナー[", "らぼみ[", "(4,16)", "(3,16)"} {
			if strings.Contains(text, leak) {
				t.Errorf("%s leaks %q", name, leak)
			}
		}
		if !strings.Contains(text, "見える他のアクター（名前[種別](x,y)）: なし") {
			t.Errorf("%s: want an empty actor list", name)
		}
	}
}

// The owner walking into the room nostarou is in becomes one said (a fact,
// once per zone, not per step); walking where nostarou is not, or where it
// may not see, says nothing.
func TestOwnerWalkingInIsSaidOnce(t *testing.T) {
	w := world.NewDefault()
	b := &Bridge{World: w, Actor: "nostarou"}
	joinOwner(t, w, world.Pos{X: 43, Y: 4}) // dining room, next to the living room
	events, cancel := w.Subscribe()
	defer cancel()
	if err := w.Move(ownerID, ownerID, world.Pos{X: 50, Y: 3}); err != nil {
		t.Fatal(err)
	}
	var said []string
	for i := 0; i < 20; i++ {
		w.Step()
	}
	for len(events) > 0 {
		if s, ok := b.toSaid(<-events); ok {
			said = append(said, s.Text)
		}
	}
	if len(said) != 1 {
		t.Fatalf("want 1 said, got %d", len(said))
	}
	if !strings.Contains(said[0], "オーナー（オーナー、"+ownerID+"）が、あなたのいる nostarou-house / リビング に入ってきた") {
		t.Errorf("said:\n%s", head(said[0]))
	}

	// nostarou is not in labomi's house: the owner walking there says nothing
	w2 := world.NewDefault()
	b2 := &Bridge{World: w2, Actor: "nostarou"}
	joinOwner(t, w2, world.Pos{X: 3, Y: 16})
	ev2, cancel2 := w2.Subscribe()
	defer cancel2()
	if _, err := w2.Join(world.Actor{ID: "nostr:guest0000000000", Name: "お客", RoomID: world.TownID, Pos: world.Pos{X: 4, Y: 16}}, ""); err != nil {
		t.Fatal(err)
	}
	for len(ev2) > 0 {
		if s, ok := b2.toSaid(<-ev2); ok {
			t.Errorf("unexpected said: %s", head(s.Text))
		}
	}
}
