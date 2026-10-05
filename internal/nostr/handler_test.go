package nostr

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/kojira/crab-town/internal/world"
)

// Signed by nostr-tools 2.17.0 finalizeEvent (secret key = 32 bytes of 0x07).
// The content exercises NIP-01 escaping: quotes, backslash, control chars,
// non-ASCII, HTML characters and U+2028 (left unescaped by JSON.stringify).
const nostrToolsVector = `{"kind":23410,"created_at":1700000000,"tags":[["p","ab"],["t","crab-town"]],"content":"こんにちは \"⚡\"\\ <&>\n\t\r\b\f\u0001\u001f` + "\u2028" + `end","pubkey":"989c0b76cb563971fdc9bef31ec06c3560f3249d6ee9e5d83c57625596e05f6f","id":"eedfb9687ce5c85a39149dd17d5446d6e28d400f219bd48900687f00ba3f956d","sig":"9bca310ae7d06b699a67d5cde9a3f27590443a5611aa9368c23524e7f4c6386b3f51c5fcf82b01b17785cc6783c5a45ed5a6b44b6e0e1806d90cff2fbb7aae4f"}`

func TestVerifyAcceptsNostrToolsEvent(t *testing.T) {
	var ev Event
	if err := json.Unmarshal([]byte(nostrToolsVector), &ev); err != nil {
		t.Fatal(err)
	}
	if err := ev.Verify(); err != nil {
		t.Fatalf("a valid nostr-tools event must verify: %v", err)
	}
}

func TestVerifyRejectsTampering(t *testing.T) {
	other := mustKey(t, 9)
	cases := map[string]func(e *Event){
		"content":   func(e *Event) { e.Content += "x" },
		"kind":      func(e *Event) { e.Kind++ },
		"tags":      func(e *Event) { e.Tags[0][1] = "cd" },
		"time":      func(e *Event) { e.CreatedAt++ },
		"sig":       func(e *Event) { e.Sig = e.Sig[:127] + flip(e.Sig[127]) },
		"pubkey":    func(e *Event) { e.PubKey = PubHex(other) },
		"id+pubkey": func(e *Event) { e.PubKey = PubHex(other); e.ID = e.ComputeID() },
		"short sig": func(e *Event) { e.Sig = e.Sig[:64] },
		"uppercase": func(e *Event) { e.ID = "A" + e.ID[1:] },
	}
	for name, mutate := range cases {
		var ev Event
		json.Unmarshal([]byte(nostrToolsVector), &ev)
		mutate(&ev)
		if err := ev.Verify(); err == nil {
			t.Errorf("%s: tampered event verified", name)
		}
	}
}

func flip(c byte) string {
	if c == '0' {
		return "1"
	}
	return "0"
}

func mustKey(t *testing.T, fill byte) *btcec.PrivateKey {
	t.Helper()
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = fill
	}
	k, _ := btcec.PrivKeyFromBytes(raw)
	return k
}

type fixture struct {
	h     *Handler
	town  *btcec.PrivateKey
	owner *btcec.PrivateKey
	guest *btcec.PrivateKey
	now   time.Time
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{town: mustKey(t, 1), owner: mustKey(t, 2), guest: mustKey(t, 3), now: time.Unix(1_800_000_000, 0)}
	f.h = &Handler{
		World: world.NewDefault(), Town: PubHex(f.town), Owner: PubHex(f.owner),
		OwnerActor: "nostarou", Window: 2 * time.Minute, Now: func() time.Time { return f.now },
	}
	return f
}

// cmd builds a signed command event from key at the fixture's clock + skew.
func (f *fixture) cmd(t *testing.T, key *btcec.PrivateKey, content string, skew time.Duration) *Event {
	t.Helper()
	ev := &Event{CreatedAt: f.now.Add(skew).Unix(), Kind: KindCommand, Tags: [][]string{{"p", PubHex(f.town)}}, Content: content}
	if err := ev.Sign(key); err != nil {
		t.Fatal(err)
	}
	return ev
}

// The bed's access tile in nostarou's bedroom: an owner-only zone.
var bedroom = fmt.Sprintf(`{"type":"move","x":%d,"y":15}`, world.NostarouX+3)

func TestOwnerMayMove(t *testing.T) {
	f := newFixture(t)
	res := f.h.Handle(f.cmd(t, f.owner, bedroom, 0))
	if res.Role != RoleOwner || res.Err != nil || !res.Reply {
		t.Fatalf("owner move: %+v", res)
	}
	a, _ := f.h.World.Actor("nostarou")
	if a.Target == nil || a.Target.X != world.NostarouX+3 || a.Target.Y != 15 {
		t.Fatalf("nostarou is not walking to the bedroom: %+v", a)
	}
}

func TestGuestMayNotMove(t *testing.T) {
	f := newFixture(t)
	for _, c := range []string{bedroom, `{"type":"move","actor":"labomi","x":23,"y":4}`, `{"type":"move","x":23,"y":4}`} {
		res := f.h.Handle(f.cmd(t, f.guest, c, 0))
		if res.Role != RoleGuest || !errors.Is(res.Err, ErrGuestOnly) {
			t.Fatalf("guest move %s: %+v", c, res)
		}
	}
	for _, id := range []string{"nostarou", "labomi"} {
		if a, _ := f.h.World.Actor(id); a.Target != nil {
			t.Fatalf("%s moved on a guest command", id)
		}
	}
}

func TestGuestMayKnock(t *testing.T) {
	f := newFixture(t)
	evs, cancel := f.h.World.Subscribe()
	defer cancel()
	res := f.h.Handle(f.cmd(t, f.guest, `{"type":"knock","room":"nostarou-house","message":"やあ"}`, 0))
	if res.Err != nil || res.Role != RoleGuest {
		t.Fatalf("guest knock: %+v", res)
	}
	ev := <-evs
	if ev.Type != "knock" || ev.By != GuestID(PubHex(f.guest)) || ev.House != world.NostarouHouse {
		t.Fatalf("knock event: %+v", ev)
	}
}

// A forged event claiming the owner's pubkey is dropped, never applied.
func TestForgedOwnerEventIsDropped(t *testing.T) {
	f := newFixture(t)
	ev := f.cmd(t, f.guest, bedroom, 0)
	ev.PubKey = PubHex(f.owner) // claim to be the owner
	ev.ID = ev.ComputeID()
	res := f.h.Handle(ev)
	if res.Reply || !errors.Is(res.Err, ErrBadSig) {
		t.Fatalf("forged owner event: %+v", res)
	}
	if a, _ := f.h.World.Actor("nostarou"); a.Target != nil {
		t.Fatal("forged event moved nostarou")
	}
}

func TestReplayIsRejected(t *testing.T) {
	f := newFixture(t)
	ev := f.cmd(t, f.owner, bedroom, 0)
	if res := f.h.Handle(ev); res.Err != nil {
		t.Fatalf("first: %+v", res)
	}
	again := *ev
	if res := f.h.Handle(&again); res.Reply || !errors.Is(res.Err, ErrReplay) {
		t.Fatalf("replayed event accepted: %+v", res)
	}
}

// A forged copy (bad sig, same id) must not burn the id of the real event.
func TestForgeryDoesNotBurnID(t *testing.T) {
	f := newFixture(t)
	ev := f.cmd(t, f.owner, bedroom, 0)
	forged := *ev
	forged.Sig = ev.Sig[:127] + flip(ev.Sig[127])
	f.h.Handle(&forged)
	if res := f.h.Handle(ev); res.Err != nil {
		t.Fatalf("real event rejected after a forgery: %+v", res)
	}
}

func TestStaleAndFutureAreRejected(t *testing.T) {
	f := newFixture(t)
	for _, skew := range []time.Duration{-3 * time.Minute, 3 * time.Minute, -time.Hour} {
		res := f.h.Handle(f.cmd(t, f.owner, bedroom, skew))
		if res.Reply || !errors.Is(res.Err, ErrStale) {
			t.Fatalf("skew %s accepted: %+v", skew, res)
		}
	}
	if res := f.h.Handle(f.cmd(t, f.owner, bedroom, -90*time.Second)); res.Err != nil {
		t.Fatalf("within window rejected: %+v", res)
	}
}

func TestOtherTownOrKindIsIgnored(t *testing.T) {
	f := newFixture(t)
	ev := &Event{CreatedAt: f.now.Unix(), Kind: KindCommand, Tags: [][]string{{"p", PubHex(f.guest)}}, Content: bedroom}
	ev.Sign(f.owner)
	if res := f.h.Handle(ev); res.Reply || !errors.Is(res.Err, ErrNotForUs) {
		t.Fatalf("not addressed to us: %+v", res)
	}
	ev = &Event{CreatedAt: f.now.Unix(), Kind: KindState, Tags: [][]string{{"p", PubHex(f.town)}}, Content: bedroom}
	ev.Sign(f.owner)
	if res := f.h.Handle(ev); res.Reply || !errors.Is(res.Err, ErrKind) {
		t.Fatalf("wrong kind: %+v", res)
	}
}

func TestSnapshotAndUnknown(t *testing.T) {
	f := newFixture(t)
	if res := f.h.Handle(f.cmd(t, f.guest, `{"type":"snapshot"}`, 0)); !res.Snapshot || res.Err != nil {
		t.Fatalf("snapshot: %+v", res)
	}
	if res := f.h.Handle(f.cmd(t, f.owner, `{"type":"teleport"}`, 0)); !errors.Is(res.Err, ErrUnknown) {
		t.Fatalf("unknown: %+v", res)
	}
	if res := f.h.Handle(f.cmd(t, f.owner, `not json`, 0)); !errors.Is(res.Err, world.ErrBadRequest) {
		t.Fatalf("bad json: %+v", res)
	}
}

// Commands older than the process start are not re-applied after a restart
// (the in-memory replay set is empty then).
func TestEventsBeforeStartAreRejected(t *testing.T) {
	f := newFixture(t)
	f.h.NotBefore = f.now.Add(-10 * time.Second)
	if res := f.h.Handle(f.cmd(t, f.owner, bedroom, -30*time.Second)); res.Reply || !errors.Is(res.Err, ErrStale) {
		t.Fatalf("pre-start event accepted: %+v", res)
	}
	if res := f.h.Handle(f.cmd(t, f.owner, bedroom, -5*time.Second)); res.Err != nil {
		t.Fatalf("post-start event rejected: %+v", res)
	}
}
