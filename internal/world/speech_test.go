package world

import (
	"strings"
	"testing"
)

func TestSpeakEmitsSayAndTruncates(t *testing.T) {
	w := NewDefault()
	ch, cancel := w.Subscribe()
	defer cancel()
	before, _ := w.Actor("nostarou")
	if err := w.Speak("nostarou", strings.Repeat("あ", MaxSpeech+5)); err != nil {
		t.Fatal(err)
	}
	ev := <-ch
	if ev.Type != EventSay || ev.Actor.ID != "nostarou" || len([]rune(ev.Message)) != MaxSpeech+1 {
		t.Fatalf("event = %+v", ev)
	}
	if after, _ := w.Actor("nostarou"); after.State != before.State || after.Pos != before.Pos {
		t.Fatalf("speak changed the actor: %+v", after)
	}
	if err := w.Speak("nostarou", ""); err != ErrBadRequest {
		t.Fatalf("empty: %v", err)
	}
	if err := w.Speak("nobody", "x"); err != ErrNoActor {
		t.Fatalf("unknown actor: %v", err)
	}
}
