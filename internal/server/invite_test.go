package server

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/kojira/crab-town/internal/world"
	"net/http/httptest"
)

var inviteTokens = Tokens{"nostarou": "test-owner-token", "labomi": "test-labomi-token", "stranger": "test-stranger-token"}

// walkIntoToilet moves the owner onto a free toilet tile and lets it arrive.
func walkIntoToilet(t *testing.T, w *world.World) {
	t.Helper()
	for _, p := range []world.Pos{{X: 6, Y: 7}, {X: 7, Y: 7}, {X: 8, Y: 7}, {X: 7, Y: 8}, {X: 6, Y: 8}, {X: 8, Y: 8}} {
		if w.Move("nostarou", "nostarou", p) == nil {
			for i := 0; i < 200; i++ {
				w.Step()
			}
			if a, _ := w.Actor("nostarou"); a.Pos == p {
				return
			}
		}
	}
	t.Fatal("could not walk into the toilet")
}

// hiddenFor returns hidden_zones and the owner actor from the first WS snapshot.
func hiddenFor(t *testing.T, base, token string) ([]string, world.Actor) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(base, "http")+"/world?token="+token, nil)
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
	if len(snap.Rooms) != 1 || len(snap.Actors) != 1 {
		t.Fatalf("bad snapshot: %s", b)
	}
	return snap.Rooms[0].HiddenZones, *snap.Actors[0]
}

// Regression: a guest listed in CRAB_INVITED, viewing with its own token while
// the owner is in the toilet, sees the invited zones (hobby, guest) but not the
// owner-only zones nor the in-use toilet. Before CRAB_INVITED existed the room
// had no invited ids, so the guest got the invited zones frosted too.
func TestInvitedGuestSeesInvitedZonesOnly(t *testing.T) {
	w := world.NewDefault()
	env := map[string]string{"CRAB_INVITED": " labomi , ,ghost"}
	ids, noToken, err := ApplyInvited(w, inviteTokens, func(k string) string { return env[k] })
	if err != nil || !slices.Equal(ids, []string{"labomi", "ghost"}) || !slices.Equal(noToken, []string{"ghost"}) {
		t.Fatalf("ApplyInvited: ids=%v noToken=%v err=%v", ids, noToken, err)
	}
	walkIntoToilet(t, w)
	ts := httptest.NewServer(New(w, inviteTokens, "", nil).Handler())
	defer ts.Close()

	hidden, owner := hiddenFor(t, ts.URL, inviteTokens["labomi"])
	for _, z := range []string{"hobby", "guest", "living"} {
		if slices.Contains(hidden, z) {
			t.Errorf("invited guest: %s must be visible, hidden=%v", z, hidden)
		}
	}
	for _, z := range []string{"bedroom", "study", "washroom", "bath", "toilet"} {
		if !slices.Contains(hidden, z) {
			t.Errorf("invited guest: %s must be hidden, hidden=%v", z, hidden)
		}
	}
	if !owner.Hidden {
		t.Errorf("invited guest saw the owner in the toilet: %+v", owner)
	}

	// a token holder who is not invited still gets hobby / guest frosted
	hidden, _ = hiddenFor(t, ts.URL, inviteTokens["stranger"])
	if !slices.Contains(hidden, "hobby") || !slices.Contains(hidden, "guest") {
		t.Errorf("uninvited viewer must not see invited zones: %v", hidden)
	}
	// the owner is the one inside the toilet, so it sees both
	hidden, _ = hiddenFor(t, ts.URL, inviteTokens["nostarou"])
	if slices.Contains(hidden, "hobby") || slices.Contains(hidden, "toilet") {
		t.Errorf("owner view wrong: %v", hidden)
	}
}

func TestApplyInvitedUnset(t *testing.T) {
	w := world.NewDefault()
	ids, _, err := ApplyInvited(w, inviteTokens, func(string) string { return "" })
	if err != nil || len(ids) != 0 {
		t.Fatalf("unset CRAB_INVITED: ids=%v err=%v", ids, err)
	}
	if s := w.ViewSnapshot("labomi"); !slices.Contains(s.Rooms[0].HiddenZones, "hobby") {
		t.Fatalf("without CRAB_INVITED labomi is not invited: %v", s.Rooms[0].HiddenZones)
	}
}
