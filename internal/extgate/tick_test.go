package extgate

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time      { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *fakeClock) add(d time.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

// tickBridge: a bridge with the tick loop off (TickOnce driven by the test)
// and a fixed clock.
func tickBridge(t *testing.T) (*Bridge, *coreConn, *fakeClock) {
	t.Helper()
	fc := newFakeCore(t)
	w, b := startBridgeWith(t, fc, func(b *Bridge) { b.TickInterval = 0 })
	_ = w
	clk := &fakeClock{t: time.Date(2026, 10, 5, 12, 0, 0, 0, time.Local)}
	b.Now = clk.now
	b.TalkQuiet, b.TickStale = 5*time.Minute, 30*time.Minute
	cc := fc.accept()
	cc.helloBind()
	waitBound(t, b)
	return b, cc, clk
}

// tickAsync runs TickOnce; if a said arrives the core side admits it.
func tickAndAdmit(t *testing.T, b *Bridge, cc *coreConn) map[string]any {
	t.Helper()
	done := make(chan bool, 1)
	go func() { sent, _ := b.TickOnce(context.Background()); done <- sent }()
	said := cc.recv()
	cc.send(map[string]any{"id": said["id"], "m": "ok", "seq": 7})
	if !<-done {
		t.Fatal("tick not reported as sent")
	}
	return said
}

func expectHeld(t *testing.T, b *Bridge, cc *coreConn, why string) {
	t.Helper()
	sent, got := b.TickOnce(context.Background())
	if sent || got != why {
		t.Fatalf("tick sent=%v why=%q, want held (%q)", sent, got, why)
	}
	cc.expectQuiet(50 * time.Millisecond)
}

func activity(cc *coreConn, id, state string) {
	cc.send(map[string]any{"m": "activity", "binding_id": testBinding, "activity_id": id, "state": state})
}

const act2 = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"

// The tick said reports time passing, recent events, the map and the operations
// -- facts only -- and the next tick waits until the agent's turn has ended.
func TestTickContentAndWaitsForAnswer(t *testing.T) {
	b, cc, clk := tickBridge(t)
	said := tickAndAdmit(t, b, cc)
	text, _ := said["text"].(string)
	if o, _ := said["origin"].(string); !strings.HasPrefix(o, "crab-town:tick:") {
		t.Errorf("origin = %v", said["origin"])
	}
	for _, want := range []string{"時間が経った", "最近の出来事: なし", "現在地: (", "ゾーン", "今取れる操作"} {
		if !strings.Contains(text, want) {
			t.Errorf("tick lacks %q", want)
		}
	}
	for _, op := range b.Operations() {
		if !strings.Contains(text, "\n- "+op.Name+": ") {
			t.Errorf("tick lacks operation %s", op.Name)
		}
	}
	for _, bad := range []string{"してください", "しなさい", "すること", "べき", "動いて"} {
		if strings.Contains(text, bad) {
			t.Errorf("tick tells the agent what to do (%q)", bad)
		}
	}

	// no answer yet: held, however much time passes (below the stale valve)
	clk.add(20 * time.Minute)
	expectHeld(t, b, cc, "previous tick not answered")
	// the agent's turn starts: still held; once it ends, the next tick goes
	activity(cc, act2, "started")
	waitNote(t, b, func(s *tickState) bool { return len(s.active) == 1 })
	expectHeld(t, b, cc, "previous tick not answered")
	activity(cc, act2, "ended")
	waitNote(t, b, func(s *tickState) bool { return !s.outstanding })
	tickAndAdmit(t, b, cc)

	// stale valve: an answer that never comes does not stop ticks forever
	clk.add(31 * time.Minute)
	tickAndAdmit(t, b, cc)
}

// While a guest is talking with the agent no tick is sent.
func TestTickHeldDuringTalk(t *testing.T) {
	b, cc, clk := tickBridge(t)
	if err := b.World.Talk("nostr:abcdef0123456789", "guest", "nostarou", "やあ"); err != nil {
		t.Fatal(err)
	}
	said := cc.recv()
	cc.send(map[string]any{"id": said["id"], "m": "ok", "seq": 1})
	waitNote(t, b, func(s *tickState) bool { return !s.lastTalk.IsZero() })
	expectHeld(t, b, cc, "talk in progress")

	// the agent answering: a running turn also holds the tick
	clk.add(6 * time.Minute)
	activity(cc, act2, "started")
	waitNote(t, b, func(s *tickState) bool { return len(s.active) == 1 })
	expectHeld(t, b, cc, "a turn is running")
	activity(cc, act2, "ended")
	waitNote(t, b, func(s *tickState) bool { return len(s.active) == 0 })

	clk.add(time.Second)
	said = tickAndAdmit(t, b, cc)
	if text, _ := said["text"].(string); !strings.Contains(text, "やあ") || !strings.Contains(text, "12:00 ") {
		t.Errorf("recent events missing the talk:\n%s", text)
	}
}

// The interval comes from env (default 600 s), and the loop really ticks at it.
func TestTickIntervalFromEnvAndLoop(t *testing.T) {
	env := map[string]string{}
	get := func(k string) string { return env[k] }
	if i, q, err := TickConfig(get); err != nil || i != 600*time.Second || q != 300*time.Second {
		t.Fatalf("defaults = %v %v %v", i, q, err)
	}
	env[EnvTickSeconds], env[EnvTalkQuietSeconds] = "42", "7"
	if i, q, err := TickConfig(get); err != nil || i != 42*time.Second || q != 7*time.Second {
		t.Fatalf("env = %v %v %v", i, q, err)
	}
	env[EnvTickSeconds] = "ten"
	if _, _, err := TickConfig(get); err == nil {
		t.Fatal("bad value accepted")
	}
	t.Setenv(EnvTickSeconds, "")
	if b := NewBridge(nil, Config{}); b.TickInterval != 600*time.Second {
		t.Fatalf("NewBridge interval = %v", b.TickInterval)
	}

	fc := newFakeCore(t)
	_, b := startBridgeWith(t, fc, func(b *Bridge) { b.TickInterval = 80 * time.Millisecond })
	cc := fc.accept()
	cc.helloBind()
	waitBound(t, b)
	start := time.Now()
	said := cc.recv()
	if o, _ := said["origin"].(string); !strings.HasPrefix(o, "crab-town:tick:") {
		t.Fatalf("first said = %v", said["origin"])
	}
	if el := time.Since(start); el > 2*time.Second {
		t.Fatalf("tick after %v", el)
	}
	cc.send(map[string]any{"id": said["id"], "m": "ok", "seq": 1})
	// unanswered: further intervals send nothing
	cc.expectQuiet(300 * time.Millisecond)
}

func waitNote(t *testing.T, b *Bridge, cond func(*tickState) bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		b.tick.mu.Lock()
		ok := cond(&b.tick)
		b.tick.mu.Unlock()
		if ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for tick state")
		}
		time.Sleep(2 * time.Millisecond)
	}
}
