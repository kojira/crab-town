package world

import (
	"strconv"
	"strings"
	"testing"
)

func TestTalkEmitsEventAndRefusesTooLong(t *testing.T) {
	w := NewDefault()
	ch, cancel := w.Subscribe()
	defer cancel()
	before, _ := w.Actor("nostarou")
	if err := w.Talk("nostr:abc", "guest", "nostarou", "やあ\r\n\x07元気？"); err != nil {
		t.Fatal(err)
	}
	ev := <-ch
	if ev.Type != EventTalk || ev.By != "nostr:abc" || ev.Role != "guest" || ev.To != "nostarou" || ev.Message != "やあ\n元気？" {
		t.Fatalf("event = %+v", ev)
	}
	if after, _ := w.Actor("nostarou"); after.Pos != before.Pos || after.State != before.State || after.Target != nil {
		t.Fatalf("talk moved the actor: %+v", after)
	}
	if err := w.Talk("nostr:abc", "guest", "nostarou", strings.Repeat("あ", MaxTalk)); err != nil {
		t.Fatalf("exactly MaxTalk: %v", err)
	}
	<-ch
	if err := w.Talk("nostr:abc", "guest", "nostarou", strings.Repeat("あ", MaxTalk+1)); err != ErrTooLong {
		t.Fatalf("MaxTalk+1: %v", err)
	}
	if err := w.Talk("nostr:abc", "guest", "nostarou", " \n "); err != ErrBadRequest {
		t.Fatalf("blank: %v", err)
	}
	if err := w.Talk("nostr:abc", "guest", "nobody", "x"); err != ErrNoActor {
		t.Fatalf("unknown actor: %v", err)
	}
}

// What an actor says reaches the public even from a private zone; where it
// stands does not.
func TestSayFromBedroomIsPublicButPositionIsNot(t *testing.T) {
	w := NewDefault()
	inBed := &Actor{ID: "nostarou", Name: "のすたろう", RoomID: TownID, Pos: Pos{29, 15}, State: StateAway, Using: "bed"}
	ev, ok := w.FilterEvent("", Event{Type: EventSay, Actor: inBed, By: "nostarou", Message: "おやすみ"})
	if !ok || ev.Message != "おやすみ" {
		t.Fatalf("say from the bedroom must reach the public: ok=%v %+v", ok, ev)
	}
	if !ev.Actor.Hidden || ev.Actor.Pos != (Pos{}) || ev.Actor.Using != "" {
		t.Fatalf("say leaked where the speaker is: %+v", ev.Actor)
	}
}

func TestWhereReportsZone(t *testing.T) {
	w := NewDefault()
	wa, ok := w.Where("nostarou")
	if !ok || wa.Actor != "のすたろう" || !strings.Contains(wa.String(), "("+strconv.Itoa(wa.Pos.X)+",") {
		t.Fatalf("where = %+v %q", wa, wa.String())
	}
	if _, ok := w.Where("nobody"); ok {
		t.Fatal("unknown actor has a whereabouts")
	}
}
