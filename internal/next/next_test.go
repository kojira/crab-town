package next

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kojira/crab-town/internal/nostr"
	"github.com/kojira/crab-town/internal/world"
)

// An unknown pubkey (in no list, no token anywhere) joins with a signature
// and appears at the spawn, named by its pubkey.
func TestUnregisteredKeyJoins(t *testing.T) {
	_, owner := newKey(t)
	_, holder := newKey(t)
	tw := newTown(t, owner, holder)
	now := time.Now()
	s := newServer(t, tw, now)
	k, pk := newKey(t)
	if code := post(t, s, k, "/join", nil, now); code != http.StatusOK {
		t.Fatalf("join: %d", code)
	}
	a, ok := tw.World.Actor(pk)
	if !ok || a.Pos != (world.Pos{X: 6, Y: 3}) || a.Pubkey != pk || a.Role != "" {
		t.Fatalf("actor after join: %+v ok=%v", a, ok)
	}
	if code := post(t, s, k, "/move", moveReq{X: 6, Y: 4}, now); code != http.StatusOK {
		t.Fatalf("move after join: %d", code)
	}
}

// Missing, malformed, forged or replayed signatures are 401 and nobody joins.
func TestBadSignatureRejected(t *testing.T) {
	_, owner := newKey(t)
	tw := newTown(t, owner, "")
	now := time.Now()
	s := newServer(t, tw, now)
	k, pk := newKey(t)

	if code := post(t, s, nil, "/join", nil, now); code != http.StatusUnauthorized {
		t.Fatalf("unsigned: %d", code)
	}
	// forged: valid event, then the pubkey swapped for someone else's
	_, other := newKey(t)
	h := authHeader(t, k, "POST", testBase+"/join", nil, now)
	raw, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(h, "Nostr "))
	var ev nostr.Event
	json.Unmarshal(raw, &ev)
	ev.PubKey = other
	ev.ID = ev.ComputeID()
	b, _ := json.Marshal(ev)
	forged := "Nostr " + base64.StdEncoding.EncodeToString(b)
	// wrong URL / method / stale
	cases := map[string]string{
		"forged":  forged,
		"garbage": "Nostr !!!",
		"bearer":  "Bearer some-token",
		"url":     authHeader(t, k, "POST", testBase+"/move", nil, now),
		"method":  authHeader(t, k, "GET", testBase+"/join", nil, now),
		"stale":   authHeader(t, k, "POST", testBase+"/join", nil, now.Add(-10*time.Minute)),
	}
	for name, hdr := range cases {
		req := httptest.NewRequest(http.MethodPost, testBase+"/join", nil)
		req.Header.Set("Authorization", hdr)
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: got %d, want 401", name, rec.Code)
		}
	}
	if tw.World.ActorCount() != 0 {
		t.Fatalf("someone joined with a bad signature")
	}
	// payload tag must cover the body actually sent
	body, _ := json.Marshal(moveReq{X: 6, Y: 4})
	req := httptest.NewRequest(http.MethodPost, testBase+"/move", bytes.NewReader(body))
	req.Header.Set("Authorization", authHeader(t, k, "POST", testBase+"/move", []byte(`{"x":6,"y":5}`), now))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("payload mismatch: %d", rec.Code)
	}
	// replay: the same signed event twice
	h = authHeader(t, k, "POST", testBase+"/join", nil, now)
	for i, want := range []int{http.StatusOK, http.StatusUnauthorized} {
		req := httptest.NewRequest(http.MethodPost, testBase+"/join", nil)
		req.Header.Set("Authorization", h)
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("replay #%d: got %d want %d", i, rec.Code, want)
		}
	}
	if _, ok := tw.World.Actor(pk); !ok {
		t.Fatal("the genuine join did not go through")
	}
}

// Applying gives nothing: the plot stays closed until the town owner approves.
// Only the owner may approve, and only an applicant.
func TestPlotNeedsApproval(t *testing.T) {
	_, owner := newKey(t)
	tw := newTown(t, owner, "")
	_, pk := newKey(t)
	_, other := newKey(t)
	if err := tw.Join(pk); err != nil {
		t.Fatal(err)
	}
	inside := world.Pos{X: 9, Y: 2}
	if err := tw.Move(pk, inside); !errors.Is(err, world.ErrForbidden) {
		t.Fatalf("vacant plot before applying: %v", err)
	}
	if err := tw.Apply(pk, "plot"); err != nil {
		t.Fatal(err)
	}
	if err := tw.Move(pk, inside); !errors.Is(err, world.ErrForbidden) {
		t.Fatalf("plot after applying, before approval: %v", err)
	}
	if r := tw.RightsOf(pk); len(r.Holds) != 0 {
		t.Fatalf("holds before approval: %+v", r)
	}
	if err := tw.Approve(pk, "plot", pk); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("self-approval: %v", err)
	}
	if err := tw.Approve(owner, "plot", other); !errors.Is(err, ErrNoApplication) {
		t.Fatalf("approving a non-applicant: %v", err)
	}
	if err := tw.Approve(owner, "plot", pk); err != nil {
		t.Fatal(err)
	}
	if err := tw.Move(pk, inside); err != nil {
		t.Fatalf("plot after approval: %v", err)
	}
	if r := tw.RightsOf(pk); !reflect.DeepEqual(r.Holds, []string{"plot"}) || r.Label != "家主" {
		t.Fatalf("rights after approval: %+v", r)
	}
	if err := tw.Apply(other, "plot"); !errors.Is(err, ErrNotVacant) {
		t.Fatalf("applying for a held plot: %v", err)
	}
}

// Whatever runs behind a key -- an agent known by name (のすたろう) or an
// anonymous browser -- the same rights give the same answers, and the
// server takes the same request from both.
func TestSameRightsForAgentAndPerson(t *testing.T) {
	_, owner := newKey(t)
	_, holder := newKey(t)
	agentK, agent := newKey(t)
	personK, person := newKey(t)
	d := testData(owner, holder)
	d.Names = []Name{{Pubkey: agent, Name: "のすたろう"}}
	tw, err := New(d, "")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	s := newServer(t, tw, now)
	try := func(pk string) []error {
		return []error{
			tw.Move(pk, world.Pos{X: 2, Y: 2}), // inside someone else's house
			tw.Move(pk, world.Pos{X: 9, Y: 2}), // a vacant plot
			tw.Approve(pk, "plot", pk),
			tw.SetInvited(pk, "a-house", []string{pk}),
		}
	}
	call := 0
	codes := func() []int { // a new second each round: a signed event is single-use
		call++
		at := now.Add(time.Duration(call) * time.Second)
		return []int{
			post(t, s, agentK, "/move", moveReq{X: 2, Y: 3}, at),
			post(t, s, personK, "/move", moveReq{X: 3, Y: 3}, at),
		}
	}
	for _, pk := range []string{agent, person} {
		if err := tw.Join(pk); err != nil {
			t.Fatal(err)
		}
	}
	ea, ep := try(agent), try(person)
	for i := range ea {
		if !errors.Is(ea[i], ep[i]) || ea[i] == nil {
			t.Errorf("check %d: agent %v / person %v", i, ea[i], ep[i])
		}
	}
	if c := codes(); c[0] != c[1] || c[0] != http.StatusForbidden {
		t.Errorf("server: agent %d / person %d", c[0], c[1])
	}
	if !reflect.DeepEqual(tw.RightsOf(agent), tw.RightsOf(person)) {
		t.Errorf("rights differ: %+v / %+v", tw.RightsOf(agent), tw.RightsOf(person))
	}
	// the holder invites both; both may now enter, by the same rule
	if err := tw.SetInvited(holder, "a-house", []string{agent, person}); err != nil {
		t.Fatal(err)
	}
	if c := codes(); c[0] != http.StatusOK || c[1] != http.StatusOK {
		t.Errorf("invited: agent %d / person %d", c[0], c[1])
	}
	a, _ := tw.World.Actor(agent)
	p, _ := tw.World.Actor(person)
	if a.Role != p.Role || a.Role != "" {
		t.Errorf("roles: %q / %q (want none)", a.Role, p.Role)
	}
	// neither may command the other (each actor moves only by itself)
	if err := tw.World.MoveExclusive(agent, person, world.Pos{X: 6, Y: 4}); !errors.Is(err, world.ErrForbidden) {
		t.Errorf("agent moved person: %v", err)
	}
	if err := tw.World.MoveExclusive(holder, agent, world.Pos{X: 6, Y: 4}); !errors.Is(err, world.ErrForbidden) {
		t.Errorf("holder moved a visitor in its house: %v", err)
	}
}

// Two actors racing for one tile or one piece of furniture: exactly one wins.
func TestSimultaneousTileAndFurniture(t *testing.T) {
	_, owner := newKey(t)
	tw := newTown(t, owner, "")
	var pks []string
	for i := 0; i < 4; i++ { // the garden has 5 free tiles
		_, pk := newKey(t)
		pks = append(pks, pk)
	}
	for _, pk := range pks {
		if err := tw.Join(pk); err != nil {
			t.Fatal(err)
		}
	}
	race := func(fn func(pk string) error) int {
		var wg sync.WaitGroup
		var mu sync.Mutex
		ok := 0
		for _, pk := range pks {
			wg.Add(1)
			go func(pk string) {
				defer wg.Done()
				if fn(pk) == nil {
					mu.Lock()
					ok++
					mu.Unlock()
				}
			}(pk)
		}
		wg.Wait()
		return ok
	}
	if n := race(func(pk string) error { return tw.Move(pk, world.Pos{X: 6, Y: 5}) }); n != 1 {
		t.Fatalf("tile race: %d winners, want 1", n)
	}
	walk(tw)
	if n := race(func(pk string) error { return tw.Interact(pk, "bench") }); n != 1 {
		t.Fatalf("furniture race: %d winners, want 1", n)
	}
	walk(tw)
	users := 0
	for _, pk := range pks {
		if a, _ := tw.World.Actor(pk); a.Using == "bench" {
			users++
		}
	}
	if users != 1 {
		t.Fatalf("bench users: %d", users)
	}
}

// Newcomers never stack on one tile, never spill into a house, and a full
// garden refuses the next one instead of putting it inside someone's home.
func TestJoinSpreadsWithoutEnteringHouses(t *testing.T) {
	_, owner := newKey(t)
	_, holder := newKey(t)
	tw := newTown(t, owner, holder)
	var wg sync.WaitGroup
	errs := make([]error, 7)
	pks := make([]string, 7)
	for i := range pks {
		_, pks[i] = newKey(t)
		wg.Add(1)
		go func(i int) { defer wg.Done(); errs[i] = tw.Join(pks[i]) }(i)
	}
	wg.Wait()
	tiles := map[world.Pos]bool{}
	joined := 0
	for i, pk := range pks {
		if errs[i] != nil {
			if !errors.Is(errs[i], world.ErrTileTaken) {
				t.Fatalf("join %d: %v", i, errs[i])
			}
			continue
		}
		a, _ := tw.World.Actor(pk)
		if a.Pos.X != 6 || tiles[a.Pos] {
			t.Fatalf("actor at %+v (outside the garden or stacked)", a.Pos)
		}
		tiles[a.Pos] = true
		joined++
	}
	if joined != 5 {
		t.Fatalf("joined %d, want 5 (the free garden tiles)", joined)
	}
}
