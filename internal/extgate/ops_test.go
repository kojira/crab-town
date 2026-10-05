package extgate

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/kojira/crab-town/internal/world"
)

// checkDeclarations applies core's hello rules (opencrab crates/extgate/src/operations.rs)
// to the operations array as it went over the wire.
func checkDeclarations(t *testing.T, raw any) []map[string]any {
	t.Helper()
	arr, ok := raw.([]any)
	if !ok || len(arr) == 0 {
		t.Fatalf("hello operations = %#v, want a non-empty array", raw)
	}
	var out []map[string]any
	prev := ""
	for _, it := range arr {
		d, ok := it.(map[string]any)
		if !ok {
			t.Fatalf("declaration not an object: %#v", it)
		}
		name, _ := d["name"].(string)
		if name == "" || !(name[0] >= 'a' && name[0] <= 'z' || name[0] >= 'A' && name[0] <= 'Z') {
			t.Fatalf("bad name %q", name)
		}
		if prev != "" && prev >= name {
			t.Fatalf("operations not sorted / duplicated: %q then %q", prev, name)
		}
		prev = name
		if s, _ := d["description"].(string); s == "" {
			t.Errorf("%s: empty description", name)
		}
		schema, ok := d["input_schema"].(map[string]any)
		if !ok {
			t.Fatalf("%s: input_schema not an object", name)
		}
		checkSchema(t, name, schema)
		for _, k := range []string{"output_schema", "callback_schema"} {
			if v, present := d[k]; !present || v != nil {
				t.Errorf("%s: %s = %#v, want null", name, k, v)
			}
		}
		auth, _ := d["authorization"].(map[string]any)
		callers, _ := auth["allowed_callers"].([]any)
		if len(callers) == 0 {
			t.Fatalf("%s: no allowed_callers", name)
		}
		var cs []string
		for _, c := range callers {
			s, _ := c.(string)
			if !slices.Contains([]string{"owner", "co_agent", "trusted", "guest"}, s) {
				t.Errorf("%s: bad caller %v", name, c)
			}
			cs = append(cs, s)
		}
		if !sort.StringsAreSorted(cs) || len(slices.Compact(slices.Clone(cs))) != len(cs) {
			t.Errorf("%s: allowed_callers not strictly ascending: %v", name, cs)
		}
		// inline: core runs it in the turn and returns the result to the model there
		if d["dispatch"] != "inline" {
			t.Errorf("%s: dispatch = %v, want inline", name, d["dispatch"])
		}
		if !slices.Contains([]any{"not_exposed", "blocked", "allowed"}, d["sub_engine"]) ||
			!slices.Contains([]any{"agent_bound", "conversation_bound"}, d["sharing"]) ||
			!slices.Contains([]any{"read_only", "state_change"}, d["effect"]) || (name == "look") != (d["effect"] == "read_only") {
			t.Errorf("%s: bad policy %v/%v/%v", name, d["sub_engine"], d["sharing"], d["effect"])
		}
		out = append(out, d)
	}
	return out
}

func checkSchema(t *testing.T, op string, node map[string]any) {
	t.Helper()
	allowed := []string{"type", "required", "properties", "enum", "items", "description", "format"}
	for k, v := range node {
		if !slices.Contains(allowed, k) {
			t.Errorf("%s: schema keyword %q not accepted by core", op, k)
		}
		switch k {
		case "properties":
			for _, sub := range v.(map[string]any) {
				checkSchema(t, op, sub.(map[string]any))
			}
		case "items":
			checkSchema(t, op, v.(map[string]any))
		}
	}
}

// hello declares exactly the operations the said text lists: move and interact.
func TestHelloDeclaresMoveAndInteract(t *testing.T) {
	fc := newFakeCore(t)
	_, b := startBridge(t, fc)
	cc := fc.accept()
	h := cc.recv()
	decls := checkDeclarations(t, h["operations"])
	var names []string
	for _, d := range decls {
		names = append(names, d["name"].(string))
	}
	if strings.Join(names, ",") != "interact,look,move" {
		t.Fatalf("declared %v", names)
	}
	mv := decls[2]["input_schema"].(map[string]any)
	props := mv["properties"].(map[string]any)
	for _, k := range []string{"x", "y"} {
		if p := props[k].(map[string]any); p["type"] != "integer" {
			t.Errorf("move.%s type = %v", k, p["type"])
		}
	}
	if len(b.Operations()) != len(decls) {
		t.Errorf("table %d, declared %d", len(b.Operations()), len(decls))
	}
}

func invoke(cc *coreConn, id, op string, payload any) map[string]any {
	cc.send(map[string]any{
		"id": id, "m": "invoke", "invocation_protocol": 1, "binding_id": testBinding,
		"declaration_digest": strings.Repeat("0", 64), "operation": op,
		"dispatch": "inline", "effect": "state_change",
		"context": map[string]any{"continuation_id": nil}, "payload": payload,
	})
	return cc.recv()
}

// move walks the agent's own actor to the chosen tile; the walk shows up as
// ordinary actor events (by = the actor itself), and the result comes back ok.
func TestInvokeMoveWalksOwnActor(t *testing.T) {
	fc := newFakeCore(t)
	w, b := startBridge(t, fc)
	events, cancel := w.Subscribe()
	defer cancel()
	cc := fc.accept()
	cc.helloBind()
	waitBound(t, b)

	to := world.Pos{X: 23, Y: 5} // garden: public
	r := invoke(cc, "call-1", "move", map[string]any{"x": 23, "y": 5})
	if r["m"] != "ok" || r["id"] != "call-1" {
		t.Fatalf("move response = %v", r)
	}
	if _, present := r["result"]; !present {
		t.Fatalf("ok without result: %v", r)
	}
	if _, present := r["seq"]; present {
		t.Fatalf("invoke ok must not carry seq: %v", r)
	}
	res := r["result"].(map[string]any)
	if res["status"] != "walking" {
		t.Errorf("result = %v", res)
	}
	select {
	case ev := <-events:
		if ev.Type != "actor" || ev.Actor == nil || ev.Actor.ID != "nostarou" || ev.By != "nostarou" ||
			ev.Actor.Target == nil || *ev.Actor.Target != to {
			t.Fatalf("event = %+v (actor %+v)", ev, ev.Actor)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no actor event")
	}
	waitFor(t, w, "arrival", func() bool {
		a, _ := w.Actor("nostarou")
		return a.Pos == to
	})
}

// Every refusal is operation_rejected (any other code makes core close the
// connection) with the reason in detail, and nothing moves.
func TestInvokeRejections(t *testing.T) {
	fc := newFakeCore(t)
	w, b := startBridge(t, fc)
	cc := fc.accept()
	cc.helloBind()
	waitBound(t, b)
	before, _ := w.Actor("nostarou")

	cases := []struct {
		name, op string
		payload  any
		detail   string
	}{
		{"out of bounds", "move", map[string]any{"x": 58, "y": 0}, "範囲外"},
		{"negative", "move", map[string]any{"x": -1, "y": 3}, "範囲外"},
		{"blocked (tree)", "move", map[string]any{"x": 24, "y": 0}, "塞がって"},
		{"forbidden (labomi's house)", "move", map[string]any{"x": 5, "y": 3}, "許可がない"},
		{"not an integer", "move", map[string]any{"x": 1.5, "y": 3}, "整数"},
		{"string coord", "move", map[string]any{"x": "3", "y": 3}, "整数"},
		{"missing y", "move", map[string]any{"x": 3}, "y がない"},
		{"payload not object", "move", []any{1, 2}, "オブジェクト"},
		{"unknown operation", "fly", map[string]any{}, "\"fly\""},
		{"no furniture", "interact", map[string]any{"furniture": "nope"}, "家具はない"},
		{"decoration", "interact", map[string]any{"furniture": "tree1"}, "使えない"},
		{"other house furniture", "interact", map[string]any{"furniture": "labomi-sofa"}, "許可がない"},
	}
	for i, c := range cases {
		id := "rej-" + string(rune('a'+i))
		r := invoke(cc, id, c.op, c.payload)
		if r["m"] != "err" || r["code"] != "operation_rejected" || r["id"] != id {
			t.Errorf("%s: response = %v", c.name, r)
			continue
		}
		if d, _ := r["detail"].(string); !strings.Contains(d, c.detail) {
			t.Errorf("%s: detail = %q, want it to mention %q", c.name, d, c.detail)
		}
	}
	// unbound binding
	cc.send(map[string]any{"id": "rej-unbound", "m": "invoke", "binding_id": "00000000-0000-4000-8000-000000000000",
		"operation": "move", "payload": map[string]any{"x": 23, "y": 5}})
	if r := cc.recv(); r["code"] != "operation_rejected" {
		t.Errorf("unbound: %v", r)
	}
	if a, _ := w.Actor("nostarou"); a.Pos != before.Pos || a.Target != nil {
		t.Errorf("a rejected operation moved the actor: %+v", a)
	}
	// still connected: a valid move goes through on the same connection
	if r := invoke(cc, "after", "move", map[string]any{"x": 23, "y": 5}); r["m"] != "ok" {
		t.Fatalf("move after rejections = %v", r)
	}
	if n := b.Client.Connects.Load(); n != 1 {
		t.Errorf("reconnected (%d hellos)", n)
	}
}

// No path: an enclosed tile is unreachable and says so.
func TestMoveUnreachable(t *testing.T) {
	w := world.New()
	w.AddRoom(&world.Room{ID: "r", Width: 7, Height: 5, Visibility: "public",
		Walls: []world.Rect{{X: 3, Y: 0, W: 1, H: 5}}})
	w.AddActor(&world.Actor{ID: "me", Name: "me", RoomID: "r", Pos: world.Pos{X: 1, Y: 1}, State: world.StateIdle})
	b := &Bridge{World: w, Actor: "me"}
	_, err := b.runMove(map[string]any{"x": json.Number("5"), "y": json.Number("1")})
	if err == nil || !strings.Contains(err.Error(), "経路がない") {
		t.Fatalf("err = %v", err)
	}
	if _, err := b.runMove(map[string]any{"x": json.Number("2"), "y": json.Number("3")}); err != nil {
		t.Fatalf("reachable move: %v", err)
	}
}

// interact walks to the furniture and uses it; the interact event reaches core
// as a said like any other.
func TestInvokeInteract(t *testing.T) {
	fc := newFakeCore(t)
	w, b := startBridge(t, fc)
	cc := fc.accept()
	cc.helloBind()
	waitBound(t, b)
	r := invoke(cc, "i-1", "interact", map[string]any{"furniture": "bookshelf"})
	if r["m"] != "ok" {
		t.Fatalf("interact = %v", r)
	}
	waitFor(t, w, "using bookshelf", func() bool {
		a, _ := w.Actor("nostarou")
		return a.Using == "bookshelf"
	})
}

// The map in said shows what the agent's actor can see and nothing else:
// labomi's private rooms (bedroom, toilet, ...) and what is inside them never
// appear; the agent's own house and the public areas do.
func TestSaidMapHidesInvisibleZones(t *testing.T) {
	fc := newFakeCore(t)
	w, b := startBridge(t, fc)
	cc := fc.accept()
	cc.helloBind()
	waitBound(t, b)
	if err := w.Talk("nostr:abcdef0123456789", "guest", "nostarou", "どこ？"); err != nil {
		t.Fatal(err)
	}
	said := cc.recv()
	text, _ := said["text"].(string)
	cc.send(map[string]any{"id": said["id"], "m": "ok", "seq": 1})
	checkMap(t, w, text)

	// look returns the same map
	r := invoke(cc, "look-1", "look", map[string]any{})
	res, _ := r["result"].(map[string]any)
	m, _ := res["map"].(string)
	if r["m"] != "ok" || m == "" || !strings.Contains(text, m) {
		t.Fatalf("look = %v", r)
	}
}

func checkMap(t *testing.T, w *world.World, text string) {
	t.Helper()
	snap := w.Snapshot()
	view := w.ViewSnapshot("nostarou")
	hidden := map[string]bool{}
	for _, id := range view.Rooms[0].HiddenZones {
		hidden[id] = true
	}
	if len(hidden) == 0 {
		t.Fatal("fixture: nostarou should not see some of labomi's zones")
	}
	r := snap.Rooms[0]
	for _, z := range r.Zones {
		line := fmt.Sprintf("%s/%s x%d-%d", z.House, z.Name, z.Rect.X, z.Rect.X+z.Rect.W-1)
		if z.House == "" {
			line = fmt.Sprintf("%s x%d-%d", z.Name, z.Rect.X, z.Rect.X+z.Rect.W-1)
		}
		if got := strings.Contains(text, line); got == hidden[z.ID] {
			t.Errorf("zone %s (hidden=%v) in said = %v", z.ID, hidden[z.ID], got)
		}
		if !hidden[z.ID] {
			continue
		}
		for _, f := range r.Furniture {
			if z.Rect.Contains(f.Pos) && strings.Contains(text, fmt.Sprintf("[%s](%d,%d)", f.Kind, f.Pos.X, f.Pos.Y)) {
				t.Errorf("furniture %s inside hidden zone %s leaked", f.ID, z.ID)
			}
		}
		for _, d := range r.Doors {
			for _, dd := range []world.Pos{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
				if z.Rect.Contains(world.Pos{X: d.X + dd.X, Y: d.Y + dd.Y}) && strings.Contains(text, fmt.Sprintf(" (%d,%d)", d.X, d.Y)) {
					t.Errorf("door (%d,%d) of hidden zone %s leaked", d.X, d.Y, z.ID)
				}
			}
		}
	}
	wa, _ := w.Where("nostarou")
	for _, want := range []string{fmt.Sprintf("現在地: (%d,%d) %s", wa.Pos.X, wa.Pos.Y, wa.Zone), "ドア: ", "ソファ[sofa](50,6)", "らぼみ[住人]("} {
		if !strings.Contains(text, want) {
			t.Errorf("map lacks %q", want)
		}
	}
}

// An actor standing in a zone the agent cannot see is not listed with a position.
func TestMapHidesActorInPrivateZone(t *testing.T) {
	w := world.NewDefault()
	b := &Bridge{World: w, Actor: "nostarou"}
	if err := w.Move("labomi", "labomi", world.Pos{X: 3, Y: 16}); err != nil { // labomi's bedroom
		t.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		w.Step()
	}
	if a, _ := w.Actor("labomi"); a.Pos != (world.Pos{X: 3, Y: 16}) {
		t.Fatalf("labomi at %v", a.Pos)
	}
	if v := b.View(); strings.Contains(v, "らぼみ[") {
		t.Errorf("hidden actor listed:\n%s", v)
	}
}
