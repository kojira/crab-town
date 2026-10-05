package extgate

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kojira/crab-town/internal/world"
)

// hello → bind → said/ok(seq) → say/ok → activity → actor moves and comes back.
func TestFlowHelloBindSaidSayActivity(t *testing.T) {
	fc := newFakeCore(t)
	w, b := startBridge(t, fc)
	events, cancel := w.Subscribe()
	defer cancel()
	cc := fc.accept()
	cc.helloBind()
	waitBound(t, b)

	// knock in the world → said to core; core answers ok with seq
	if err := w.Knock("kojira", "nostarou-house", "あそぼ"); err != nil {
		t.Fatal(err)
	}
	said := cc.recv()
	if said["m"] != "said" || said["binding_id"] != testBinding || said["author_id"] != "crab-town" {
		t.Fatalf("said = %v", said)
	}
	if !strings.Contains(said["text"].(string), "あそぼ") || said["author_label"] != "kojira" {
		t.Fatalf("said text/label = %v", said)
	}
	if o, _ := said["origin"].(string); !strings.HasPrefix(o, "crab-town:knock:") {
		t.Fatalf("said origin = %v", said["origin"])
	}
	if a, ok := said["attachments"].([]any); !ok || len(a) != 0 {
		t.Fatalf("said attachments = %v", said["attachments"])
	}
	cc.send(map[string]any{"id": said["id"], "m": "ok", "seq": 1})

	// a direct Said returns the seq core assigned
	got := make(chan *int64, 1)
	go func() {
		seq, err := b.Client.Said(context.Background(), Said{Origin: "o-2", Text: "hi"})
		if err != nil {
			t.Errorf("Said: %v", err)
		}
		got <- seq
	}()
	s2 := cc.recv()
	if s2["origin"] != "o-2" {
		t.Fatalf("said 2 = %v", s2)
	}
	cc.send(map[string]any{"id": s2["id"], "m": "ok", "seq": 7})
	if seq := <-got; seq == nil || *seq != 7 {
		t.Fatalf("seq = %v, want 7", seq)
	}

	// say → ok, and a bubble event in the world
	delivery := "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
	cc.send(map[string]any{"id": delivery, "m": "say", "binding_id": testBinding, "payload": map[string]any{"text": "よっ⚡"}})
	ok := cc.recv()
	if ok["m"] != "ok" || ok["id"] != delivery {
		t.Fatalf("say response = %v", ok)
	}
	waitEvent(t, events, func(ev world.Event) bool {
		return ev.Type == world.EventSay && ev.Message == "よっ⚡" && ev.Actor.ID == "nostarou"
	})

	// empty say text → external_rejected
	cc.send(map[string]any{"id": "x-1", "m": "say", "binding_id": testBinding, "payload": map[string]any{"text": ""}})
	if r := cc.recv(); r["m"] != "err" || r["code"] != "external_rejected" || r["id"] != "x-1" {
		t.Fatalf("empty say response = %v", r)
	}

	// activity is a notice of a turn starting / ending: nothing moves
	start, _ := w.Actor("nostarou")
	cc.send(map[string]any{"m": "activity", "binding_id": testBinding, "activity_id": testActivity, "state": "started"})
	cc.expectQuiet(150 * time.Millisecond)
	if a, _ := w.Actor("nostarou"); a.Pos != start.Pos || a.Using != start.Using || a.Target != nil {
		t.Fatalf("activity started moved the actor: %+v -> %+v", start, a)
	}
	cc.send(map[string]any{"m": "activity", "binding_id": testBinding, "activity_id": testActivity, "state": "ended"})
	cc.expectQuiet(150 * time.Millisecond)
	if a, _ := w.Actor("nostarou"); a.Pos != start.Pos || a.Using != start.Using || a.Target != nil {
		t.Fatalf("activity ended moved the actor: %+v -> %+v", start, a)
	}
}

// A visitor using furniture is forwarded; a turn does not move the actor away from it.
func TestInteractSaidAndActivityKeepsFurniture(t *testing.T) {
	fc := newFakeCore(t)
	w, b := startBridge(t, fc)
	cc := fc.accept()
	cc.helloBind()
	waitBound(t, b)

	if err := w.Interact("nostarou", "nostarou", "bookshelf"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, w, "at bookshelf", func() bool { a, _ := w.Actor("nostarou"); return a.Using == "bookshelf" })
	said := cc.recv()
	if said["m"] != "said" || !strings.Contains(said["text"].(string), "本棚") {
		t.Fatalf("said = %v", said)
	}
	cc.send(map[string]any{"id": said["id"], "m": "ok", "seq": nil})

	cc.send(map[string]any{"m": "activity", "binding_id": testBinding, "activity_id": testActivity, "state": "started"})
	cc.expectQuiet(150 * time.Millisecond) // no said: nothing happened in the world
	if a, _ := w.Actor("nostarou"); a.Using != "bookshelf" {
		t.Fatalf("activity took the actor off the bookshelf: %+v", a)
	}
}

// A Nostr talk to the actor reaches core as a said that reports what happened,
// where the actor is, and the operations crab-town accepts -- and nothing that
// tells the agent what to do.
func TestTalkSaidDescribesEventAndOperations(t *testing.T) {
	fc := newFakeCore(t)
	w, b := startBridge(t, fc)
	cc := fc.accept()
	cc.helloBind()
	waitBound(t, b)

	if err := w.Talk("nostr:abcdef0123456789", "guest", "nostarou", "元気？"); err != nil {
		t.Fatal(err)
	}
	said := cc.recv()
	text, _ := said["text"].(string)
	t.Logf("said text:\n%s", text)
	if o, _ := said["origin"].(string); !strings.HasPrefix(o, "crab-town:talk:") {
		t.Fatalf("origin = %v", said["origin"])
	}
	wa, _ := w.Where("nostarou")
	for _, want := range []string{
		"出来事: nostr:abcdef0123456789（guest） が のすたろう に話しかけた", "本文: 元気？",
		"の現在地: " + wa.String(), "今取れる操作",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("said text lacks %q", want)
		}
	}
	// the operations listed are exactly the declared table
	ops := b.Operations()
	if len(ops) == 0 {
		t.Fatal("no operations")
	}
	for _, op := range ops {
		if !strings.Contains(text, "\n- "+op.Name+": ") {
			t.Errorf("said text lacks operation %q", op.Name)
		}
	}
	if n := strings.Count(text, "\n- "); n != len(ops) {
		t.Errorf("listed %d operations, table has %d", n, len(ops))
	}
	// no instructions: crab-town reports, the agent decides
	for _, bad := range []string{"してください", "返事", "返答", "しなさい", "すること", "べき"} {
		if strings.Contains(text, bad) {
			t.Errorf("said text tells the agent what to do (%q)", bad)
		}
	}
	cc.send(map[string]any{"id": said["id"], "m": "ok", "seq": 3})

	// a talk to someone else is not the agent's business
	if err := w.Talk("nostr:abcdef0123456789", "guest", "labomi", "hi"); err != nil {
		t.Fatal(err)
	}
	cc.expectQuiet(150 * time.Millisecond)
}

// Broken frames close the connection; the gateway then reconnects and says hello again.
func TestBrokenFrameClosesAndReconnects(t *testing.T) {
	cases := map[string][]byte{
		"invalid json":     []byte("{\"m\":\n"),
		"not an object":    []byte("[1,2]\n"),
		"invalid utf8":     append([]byte("{\"m\":\"x\",\"p\":\""), 0xff, '"', '}', '\n'),
		"duplicate member": []byte(`{"m":"activity","m":"activity","binding_id":"` + testBinding + `","activity_id":"` + testActivity + `","state":"started"}` + "\n"),
		"nested duplicate": []byte(`{"id":"d1","m":"say","binding_id":"` + testBinding + `","payload":{"text":"a","text":"b"}}` + "\n"),
		"too large no LF":  []byte(`{"m":"x","pad":"` + strings.Repeat("a", MaxFrame) + `"}`),
		"too large w/ LF":  []byte(`{"m":"x","pad":"` + strings.Repeat("a", MaxFrame-17) + `"}` + "\n"),
		"trailing data":    []byte(`{"m":"x"} {"m":"y"}` + "\n"),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			fc := newFakeCore(t)
			w, b := startBridge(t, fc)
			cc := fc.accept()
			cc.helloBind()
			waitBound(t, b)
			go cc.c.Write(raw) // may block on a large frame until the peer closes
			cc.expectClosed()
			cc2 := fc.accept()
			cc2.helloBind()
			waitConnects(t, b, 2)
			if name == "duplicate member" {
				if a, _ := w.Actor("nostarou"); a.Target != nil || a.Using != "" {
					t.Fatalf("rejected frame moved the actor: %+v", a)
				}
			}
		})
	}
}

// A frame of exactly MaxFrame bytes (LF included) is accepted; the connection stays.
func TestExactMaxFrameAccepted(t *testing.T) {
	fc := newFakeCore(t)
	_, b := startBridge(t, fc)
	cc := fc.accept()
	cc.helloBind()
	waitBound(t, b)
	head := `{"m":"noop","pad":"`
	tail := `"}` + "\n"
	frame := head + strings.Repeat("a", MaxFrame-len(head)-len(tail)) + tail
	if len(frame) != MaxFrame {
		t.Fatalf("frame len %d", len(frame))
	}
	cc.sendRaw([]byte(frame))
	cc.expectQuiet(150 * time.Millisecond) // unknown m without id: no reply, keep
	cc.send(map[string]any{"id": "u1", "m": "turn_failed"})
	if r := cc.recv(); r["m"] != "err" || r["code"] != "unknown_message" || r["id"] != "u1" {
		t.Fatalf("unknown message response = %v", r)
	}
}

// Core going away: reconnect with backoff and hello again.
func TestReconnectAfterCoreClose(t *testing.T) {
	fc := newFakeCore(t)
	_, b := startBridge(t, fc)
	cc := fc.accept()
	cc.helloBind()
	waitBound(t, b)
	cc.c.Close()
	cc2 := fc.accept()
	cc2.helloBind()
	waitBound(t, b)
	waitConnects(t, b, 2)
}

// The gateway keeps retrying (with backoff) until core starts listening.
func TestConnectRetriesUntilCoreListens(t *testing.T) {
	dir, err := os.MkdirTemp("", "eg")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "core.sock")
	cfg := Config{Socket: path, InstanceID: testInstance, Revision: 1, ConfigDigest: testDigest, AuthorID: "a", Actor: "nostarou"}
	b := NewBridge(world.NewDefault(), cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.Run(ctx)
	time.Sleep(120 * time.Millisecond) // several failed dials
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	fc := &fakeCore{t: t, ln: ln, path: path, conns: make(chan net.Conn, 4)}
	go func() {
		c, err := ln.Accept()
		if err == nil {
			fc.conns <- c
		}
	}()
	fc.accept().helloBind()
}

// hello rejected by core (err) → close, then retry.
func TestHelloRejectedRetries(t *testing.T) {
	fc := newFakeCore(t)
	_, b := startBridge(t, fc)
	cc := fc.accept()
	h := cc.recv()
	cc.send(map[string]any{"id": h["id"], "m": "err", "code": "revision_mismatch", "detail": nil})
	cc.expectClosed()
	fc.accept().helloBind()
	waitConnects(t, b, 1)
}

func TestSaidErrAndUnknownResponse(t *testing.T) {
	fc := newFakeCore(t)
	_, b := startBridge(t, fc)
	cc := fc.accept()
	cc.helloBind()
	waitBound(t, b)
	errc := make(chan error, 1)
	go func() {
		_, err := b.Client.Said(context.Background(), Said{Origin: "o", Text: "t"})
		errc <- err
	}()
	s := cc.recv()
	cc.send(map[string]any{"id": s["id"], "m": "err", "code": "binding_closed", "detail": nil})
	if se, ok := (<-errc).(*SaidError); !ok || se.Code != "binding_closed" {
		t.Fatalf("said err = %v", se)
	}
	cc.send(map[string]any{"id": "nobody", "m": "ok"}) // unknown id → response_invalid → close
	cc.expectClosed()
}

func TestLoadConfig(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	if c, err := LoadConfig(env(nil)); c != nil || err != nil {
		t.Fatalf("unset: %v %v", c, err)
	}
	if _, err := LoadConfig(env(map[string]string{EnvSocket: "/x"})); err == nil {
		t.Fatal("partial config accepted")
	}
	full := map[string]string{EnvSocket: "/x.sock", EnvInstanceID: testInstance, EnvRevision: "2",
		EnvConfigDigest: testDigest, EnvAuthorID: "crab-town"}
	c, err := LoadConfig(env(full))
	if err != nil || c.Revision != 2 || c.Actor != "nostarou" {
		t.Fatalf("full: %+v %v", c, err)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "eg.json")
	b, _ := json.Marshal(map[string]any{"socket": "/y.sock", "instance_id": testInstance, "revision": 3,
		"config_digest": testDigest, "author_id": "a", "actor": "labomi"})
	os.WriteFile(p, b, 0o600)
	c, err = LoadConfig(env(map[string]string{EnvConfigFile: p, EnvRevision: "4"}))
	if err != nil || c.Socket != "/y.sock" || c.Revision != 4 || c.Actor != "labomi" {
		t.Fatalf("file: %+v %v", c, err)
	}
	full[EnvInstanceID] = strings.ToUpper(testInstance)
	if _, err := LoadConfig(env(full)); err == nil {
		t.Fatal("uppercase uuid accepted")
	}
}

func waitEvent(t *testing.T, ch <-chan world.Event, match func(world.Event) bool) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case ev := <-ch:
			if match(ev) {
				return
			}
		case <-deadline:
			t.Fatal("event not seen")
		}
	}
}
