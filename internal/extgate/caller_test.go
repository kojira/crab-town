package extgate

import (
	"testing"
	"time"
)

// The town owner talking to the agent must start an owner turn in core:
// said carries caller {"role":"owner"}. Without it core runs the turn as an
// untrusted agent and every owner-only tool is refused ("requires owner").
func TestOwnerTalkSaidCarriesOwnerCaller(t *testing.T) {
	fc := newFakeCore(t)
	w, b := startBridge(t, fc)
	cc := fc.accept()
	cc.helloBind()
	waitBound(t, b)

	if err := w.Talk("nostr:b3e43e8cc7e6dff2", "owner", "nostarou", "おう"); err != nil {
		t.Fatal(err)
	}
	said := cc.recv()
	caller, ok := said["caller"].(map[string]any)
	if !ok {
		t.Fatalf("owner talk said has no caller: %v", said["caller"])
	}
	if caller["role"] != "owner" || len(caller) != 1 {
		t.Fatalf("caller = %v, want {role: owner}", caller)
	}
	cc.send(map[string]any{"id": said["id"], "m": "ok", "seq": 1})
}

// A guest's talk (and anything not started by the owner) never claims owner:
// no caller member, which core reads as an untrusted agent turn.
func TestGuestTalkSaidHasNoCaller(t *testing.T) {
	fc := newFakeCore(t)
	w, b := startBridge(t, fc)
	cc := fc.accept()
	cc.helloBind()
	waitBound(t, b)

	if err := w.Talk("nostr:abcdef0123456789", "guest", "nostarou", "やあ"); err != nil {
		t.Fatal(err)
	}
	said := cc.recv()
	if c, present := said["caller"]; present {
		t.Fatalf("guest talk said has caller %v", c)
	}
	cc.send(map[string]any{"id": said["id"], "m": "ok", "seq": 1})

	// a knock carries no verified role either
	if err := w.Knock("nostr:abcdef0123456789", "nostarou-house", "こんにちは"); err != nil {
		t.Fatal(err)
	}
	said = cc.recv()
	if c, present := said["caller"]; present {
		t.Fatalf("knock said has caller %v", c)
	}
	cc.send(map[string]any{"id": said["id"], "m": "ok", "seq": 2})
	cc.expectQuiet(100 * time.Millisecond)
}
