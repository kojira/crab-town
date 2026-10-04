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

func post(t *testing.T, url, by, body string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("POST", url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if by != "" {
		req.Header.Set(RequesterHeader, by)
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
	ts := httptest.NewServer(New(w, "", fstest.MapFS{"index.html": {Data: []byte("hi")}}).Handler())
	defer ts.Close()

	cases := []struct {
		path, by, body string
		want           int
	}{
		{"/actor/interact", "nostarou", `{"actor":"nostarou","furniture":"window"}`, 200},
		{"/actor/interact", "", `{"actor":"nostarou","furniture":"window"}`, 403},
		{"/actor/interact", "nostarou", `{"actor":"nostarou","furniture":"nope"}`, 404},
		{"/actor/move", "nostarou", `{"actor":"nostarou","x":99,"y":0}`, 400},
		{"/actor/move", "guest", `{"actor":"nostarou","x":1,"y":1}`, 403},
		{"/actor/move", "nostarou", `not json`, 400},
		{"/actor/knock", "guest", `{"room":"nostarou-room","message":"hi"}`, 200},
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
	ts := httptest.NewServer(New(w, hook.URL, nil).Handler())
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
	ts := httptest.NewServer(New(w, "", nil).Handler())
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
	if snap.Type != "snapshot" || len(snap.Rooms) != 1 || len(snap.Rooms[0].Zones) != 9 || len(snap.Rooms[0].Furniture) == 0 {
		t.Fatalf("bad snapshot: %s", b)
	}

	// viewer writes are ignored (read-only)
	c.Write(ctx, websocket.MessageText, []byte(`{"actor":"nostarou","x":0,"y":0}`))

	post(t, ts.URL+"/actor/move", "nostarou", `{"actor":"nostarou","x":22,"y":5}`)
	_, b, err = c.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var ev world.Event
	json.Unmarshal(b, &ev)
	if ev.Type != "actor" || ev.Actor == nil || ev.Actor.Target == nil || *ev.Actor.Target != (world.Pos{X: 22, Y: 5}) {
		t.Fatalf("unexpected event: %s", b)
	}
}

// readSnap dials /world?viewer=... and returns the actor from the snapshot.
func readSnapActor(t *testing.T, base, viewer string) world.Actor {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(base, "http")+"/world?"+ViewerParam+"="+viewer, nil)
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
	if len(snap.Actors) != 1 {
		t.Fatalf("bad snapshot: %s", b)
	}
	return *snap.Actors[0]
}

// WS output is filtered per viewer: in bed (owner-only bedroom) the public sees
// only a hidden actor, the owner sees the real position.
func TestWSFiltersPrivateZones(t *testing.T) {
	w := world.NewDefault()
	ts := httptest.NewServer(New(w, "", nil).Handler())
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
		if ev.Type == "interact" || (ev.Actor != nil && !ev.Actor.Hidden && ev.Actor.Pos.Y >= 12) {
			t.Fatalf("public stream leaked: %s", b)
		}
	}
	if a := readSnapActor(t, ts.URL, ""); !a.Hidden || a.Using != "" || a.Pos != (world.Pos{}) {
		t.Fatalf("anonymous snapshot leaked: %+v", a)
	}
	if a := readSnapActor(t, ts.URL, "nostarou"); a.Hidden || a.Using != "bed" {
		t.Fatalf("owner snapshot wrong: %+v", a)
	}
}
