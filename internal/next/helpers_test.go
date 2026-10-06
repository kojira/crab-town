package next

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/kojira/crab-town/internal/nostr"
	"github.com/kojira/crab-town/internal/world"
)

func newKey(t *testing.T) (*btcec.PrivateKey, string) {
	t.Helper()
	k, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	return k, nostr.PubHex(k)
}

// authHeader signs a NIP-98 event for method + url (+ body).
func authHeader(t *testing.T, k *btcec.PrivateKey, method, url string, body []byte, at time.Time) string {
	t.Helper()
	tags := [][]string{{"u", url}, {"method", method}}
	if len(body) > 0 {
		sum := sha256.Sum256(body)
		tags = append(tags, []string{"payload", hex.EncodeToString(sum[:])})
	}
	ev := &nostr.Event{CreatedAt: at.Unix(), Kind: KindHTTPAuth, Tags: tags}
	if err := ev.Sign(k); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(ev)
	return "Nostr " + base64.StdEncoding.EncodeToString(b)
}

// testData: a 12x6 room "town". A house "a-house" (x=0..5, a door at (5,2))
// held by holder, and a vacant plot "plot" (x=7..11, door at (7,2)). The
// garden is the column x=6. A usable bench in the garden at (6,0), used from (6,1).
func testData(owner, holder string) *Data {
	walls := []world.Rect{
		{X: 0, Y: 0, W: 6, H: 1}, {X: 0, Y: 5, W: 6, H: 1}, {X: 0, Y: 0, W: 1, H: 6}, {X: 5, Y: 0, W: 1, H: 6},
		{X: 7, Y: 0, W: 5, H: 1}, {X: 7, Y: 5, W: 5, H: 1}, {X: 7, Y: 0, W: 1, H: 6}, {X: 11, Y: 0, W: 1, H: 6},
	}
	return &Data{
		Owner: owner,
		Spawn: Spawn{Room: "town", Pos: world.Pos{X: 6, Y: 3}},
		Rooms: []*world.Room{{
			ID: "town", Width: 12, Height: 6, Visibility: world.VisPublic,
			Houses: []*world.House{
				{ID: "a-house", Owner: holder, Invited: []string{}, Rect: world.Rect{X: 0, Y: 0, W: 6, H: 6}},
				{ID: "plot", Owner: "", Invited: []string{}, Rect: world.Rect{X: 7, Y: 0, W: 5, H: 6}},
			},
			Zones: []*world.Zone{
				{ID: "a-room", Name: "Aの部屋", Visibility: world.VisOwner, Rect: world.Rect{X: 1, Y: 1, W: 4, H: 4}, House: "a-house"},
				{ID: "garden", Name: "庭", Visibility: world.VisPublic, Rect: world.Rect{X: 6, Y: 0, W: 1, H: 6}},
				{ID: "plot-room", Name: "空き地", Visibility: world.VisInvited, Rect: world.Rect{X: 8, Y: 1, W: 3, H: 4}, House: "plot"},
			},
			Walls: walls,
			Doors: []world.Pos{{X: 5, Y: 2}, {X: 7, Y: 2}},
			Furniture: []*world.Furniture{
				{ID: "bench", Kind: world.KindSofa, Label: "ベンチ", Function: "rest", State: world.StateIdle, Pos: world.Pos{X: 6, Y: 0}, Size: world.Size{W: 1, H: 1}, Access: world.Pos{X: 6, Y: 1}},
			},
		}},
	}
}

func newTown(t *testing.T, owner, holder string) *Town {
	t.Helper()
	tw, err := New(testData(owner, holder), "")
	if err != nil {
		t.Fatal(err)
	}
	return tw
}

// walk steps the world until nobody walks (bounded).
func walk(tw *Town) {
	for i := 0; i < 50; i++ {
		tw.World.Step()
	}
}

const testBase = "http://town.test"

func newServer(t *testing.T, tw *Town, now time.Time) *Server {
	return &Server{Town: tw, Auth: &Verifier{BaseURL: testBase, Window: time.Minute, Now: func() time.Time { return now }}}
}

// post calls an endpoint as k (nil = unsigned) and returns the status.
func post(t *testing.T, s *Server, k *btcec.PrivateKey, path string, body any, now time.Time) int {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, testBase+path, bytes.NewReader(b))
	if k != nil {
		req.Header.Set("Authorization", authHeader(t, k, "POST", testBase+path, b, now))
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec.Code
}
