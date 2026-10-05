package extgate

import (
	"testing"
	"time"

	"github.com/kojira/crab-town/internal/world"
)

// A say right after a talk is marked as answering that talker (reply_to =
// the talk's by); with no recent talk it answers nobody in particular.
func TestSayAfterTalkCarriesReplyTo(t *testing.T) {
	fc := newFakeCore(t)
	w, b := startBridge(t, fc)
	events, cancel := w.Subscribe()
	defer cancel()
	cc := fc.accept()
	cc.helloBind()
	waitBound(t, b)
	b.TalkQuiet = 5 * time.Minute

	say := func(id, text string) {
		cc.send(map[string]any{"id": id, "m": "say", "binding_id": testBinding, "payload": map[string]any{"text": text}})
		if ok := cc.recv(); ok["m"] != "ok" {
			t.Fatalf("say response = %v", ok)
		}
	}
	// no talk yet: no reply_to
	say("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "ひとりごと")
	waitEvent(t, events, func(ev world.Event) bool {
		return ev.Type == world.EventSay && ev.Message == "ひとりごと" && ev.ReplyTo == ""
	})

	if err := w.Talk("nostr:abcdef0123456789", "guest", "nostarou", "元気？"); err != nil {
		t.Fatal(err)
	}
	said := cc.recv()
	cc.send(map[string]any{"id": said["id"], "m": "ok", "seq": 1})
	say("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", "元気だよ")
	waitEvent(t, events, func(ev world.Event) bool {
		return ev.Type == world.EventSay && ev.Message == "元気だよ" && ev.ReplyTo == "nostr:abcdef0123456789"
	})

	// long after the talk: no longer a reply
	b.Now = func() time.Time { return time.Now().Add(10 * time.Minute) }
	say("cccccccc-cccc-4ccc-8ccc-cccccccccccc", "またね")
	waitEvent(t, events, func(ev world.Event) bool {
		return ev.Type == world.EventSay && ev.Message == "またね" && ev.ReplyTo == ""
	})
}
