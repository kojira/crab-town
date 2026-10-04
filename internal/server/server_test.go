package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
	"github.com/kojira/crab-town/internal/world"
)

// Test-only tokens (generated per test binary; never real secrets).
var testTokens = Tokens{"nostarou": "test-owner-token", "guest": "test-guest-token"}

// post calls a mutating API as actor `by` ("" = no token, "!bad" = wrong token).
func post(t *testing.T, url, by, body string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("POST", url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	switch {
	case by == "!bad":
		req.Header.Set("Authorization", "Bearer not-a-token")
	case by != "":
		req.Header.Set("Authorization", "Bearer "+testTokens[by])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

func TestHTTPStatusCodes(t *testing.T) {
	w := world.NewDefault()
	ts := httptest.NewServer(New(w, testTokens, "", fstest.MapFS{"index.html": {Data: []byte("hi")}}).Handler())
	defer ts.Close()

	cases := []struct {
		path, by, body string
		want           int
	}{
		{"/actor/interact", "nostarou", `{"actor":"nostarou","furniture":"window"}`, 200},
		{"/actor/interact", "", `{"actor":"nostarou","furniture":"window"}`, 401},
		{"/actor/interact", "!bad", `{"actor":"nostarou","furniture":"window"}`, 401},
		{"/actor/move", "", `{"actor":"nostarou","x":48,"y":5}`, 401},
		{"/actor/move", "!bad", `{"actor":"nostarou","x":48,"y":5}`, 401},
		{"/actor/knock", "", `{"room":"nostarou-house","message":"hi"}`, 401},
		{"/actor/knock", "!bad", `{"room":"nostarou-house","message":"hi"}`, 401},
		{"/actor/interact", "nostarou", `{"actor":"nostarou","furniture":"nope"}`, 404},
		{"/actor/move", "nostarou", `{"actor":"nostarou","x":99,"y":0}`, 400},
		{"/actor/move", "guest", `{"actor":"nostarou","x":27,"y":1}`, 403},
		{"/actor/move", "nostarou", `not json`, 400},
		{"/actor/knock", "guest", `{"room":"nostarou-house","message":"hi"}`, 200},
	}
	for _, c := range cases {
		if got := post(t, ts.URL+c.path, c.by, c.body).StatusCode; got != c.want {
			t.Errorf("%s by %q %s: got %d want %d", c.path, c.by, c.body, got, c.want)
		}
	}
	resp, err := http.Get(ts.URL + "/")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("GET /: %v %v", err, resp)
	}
}

func TestWebhookOnInteract(t *testing.T) {
	got := make(chan world.Event, 4)
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ev world.Event
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &ev)
		got <- ev
	}))
	defer hook.Close()

	w := world.NewDefault()
	ts := httptest.NewServer(New(w, testTokens, hook.URL, nil).Handler())
	defer ts.Close()

	post(t, ts.URL+"/actor/interact", "nostarou", `{"actor":"nostarou","furniture":"bed"}`)
	for i := 0; i < 100; i++ {
		w.Step()
	}
	select {
	case ev := <-got:
		if ev.Type != "interact" || ev.Furniture == nil || ev.Furniture.ID != "bed" {
			t.Fatalf("unexpected webhook payload %+v", ev)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("webhook not called")
	}
}

func TestWSSnapshotAndEvents(t *testing.T) {
	w := world.NewDefault()
	ts := httptest.NewServer(New(w, testTokens, "", nil).Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/world", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()

	_, b, err := c.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var snap world.Snapshot
	json.Unmarshal(b, &snap)
	if snap.Type != "snapshot" || len(snap.Rooms) != 1 || len(snap.Rooms[0].Zones) != 21 || len(snap.Rooms[0].Houses) != 2 || len(snap.Rooms[0].Furniture) == 0 {
		t.Fatalf("bad snapshot: %s", b)
	}

	// viewer writes are ignored (read-only)
	c.Write(ctx, websocket.MessageText, []byte(`{"actor":"nostarou","x":26,"y":0}`))

	post(t, ts.URL+"/actor/move", "nostarou", `{"actor":"nostarou","x":48,"y":5}`)
	_, b, err = c.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var ev world.Event
	json.Unmarshal(b, &ev)
	if ev.Type != "actor" || ev.Actor == nil || ev.Actor.Target == nil || *ev.Actor.Target != (world.Pos{X: 48, Y: 5}) {
		t.Fatalf("unexpected event: %s", b)
	}
}

// snapActor returns actor id from a snapshot (fails the test if absent).
func snapActor(t *testing.T, snap world.Snapshot, id string) world.Actor {
	t.Helper()
	for _, a := range snap.Actors {
		if a.ID == id {
			return *a
		}
	}
	t.Fatalf("no actor %s in snapshot", id)
	return world.Actor{}
}

// readSnapActor dials /world+query (with optional headers) and returns the
// actor and viewer from the first snapshot.
func readSnapActor(t *testing.T, base, query string, h http.Header) (world.Actor, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(base, "http")+"/world"+query, &websocket.DialOptions{HTTPHeader: h})
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	_, b, err := c.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var snap world.Snapshot
	json.Unmarshal(b, &snap)
	return snapActor(t, snap, "nostarou"), snap.Viewer
}

// WS output is filtered per viewer: in bed (owner-only bedroom) the public sees
// only a hidden actor, the owner sees the real position.
func TestWSFiltersPrivateZones(t *testing.T) {
	w := world.NewDefault()
	ts := httptest.NewServer(New(w, testTokens, "", nil).Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pub, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/world", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pub.CloseNow()
	pub.Read(ctx) // snapshot

	post(t, ts.URL+"/actor/interact", "nostarou", `{"actor":"nostarou","furniture":"bed"}`)
	for i := 0; i < 100; i++ {
		w.Step()
	}
	if a, _ := w.Actor("nostarou"); a.Using != "bed" {
		t.Fatalf("did not reach bed: %+v", a)
	}
	// drain the public stream: no message may reveal a bedroom tile or the bed
	for {
		rctx, rc := context.WithTimeout(ctx, 200*time.Millisecond)
		_, b, err := pub.Read(rctx)
		rc()
		if err != nil {
			break
		}
		var ev world.Event
		json.Unmarshal(b, &ev)
		if ev.Type == "interact" || (ev.Actor != nil && ev.Actor.ID == "nostarou" && !ev.Actor.Hidden && ev.Actor.Pos.Y >= 12) {
			t.Fatalf("public stream leaked: %s", b)
		}
	}
	if a, _ := readSnapActor(t, ts.URL, "", nil); !a.Hidden || a.Using != "" || a.Pos != (world.Pos{}) {
		t.Fatalf("anonymous snapshot leaked: %+v", a)
	}
	if a, _ := readSnapActor(t, ts.URL, "?token="+testTokens["nostarou"], nil); a.Hidden || a.Using != "bed" {
		t.Fatalf("owner snapshot wrong: %+v", a)
	}
}
