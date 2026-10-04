package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/kojira/crab-town/internal/world"
)

// The old self-declared identities no longer grant anything: X-Crab-Id without a
// token is 401, and ?viewer=nostarou on the WebSocket is a public viewer.
func TestSelfDeclaredIdentityIgnored(t *testing.T) {
	w := world.NewDefault()
	ts := httptest.NewServer(New(w, testTokens, "", nil).Handler())
	defer ts.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/actor/move", strings.NewReader(`{"actor":"nostarou","x":48,"y":5}`))
	req.Header.Set("X-Crab-Id", "nostarou")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("X-Crab-Id only: got %d want 401", resp.StatusCode)
	}

	if _, viewer := readSnapActor(t, ts.URL, "?viewer=nostarou", nil); viewer != "" {
		t.Fatalf("?viewer= must not pick the viewer, got %q", viewer)
	}
}

// The WS viewer comes from the token: ?token= or Authorization header.
// The guest token never yields the owner view.
func TestWSViewerFromToken(t *testing.T) {
	w := world.NewDefault()
	ts := httptest.NewServer(New(w, testTokens, "", nil).Handler())
	defer ts.Close()

	if _, v := readSnapActor(t, ts.URL, "?token="+testTokens["guest"], nil); v != "guest" {
		t.Fatalf("query token: viewer %q want guest", v)
	}
	h := http.Header{"Authorization": {"Bearer " + testTokens["nostarou"]}}
	if _, v := readSnapActor(t, ts.URL, "", h); v != "nostarou" {
		t.Fatalf("header token: viewer %q want nostarou", v)
	}
	if _, v := readSnapActor(t, ts.URL, "", nil); v != "" {
		t.Fatalf("no token: viewer %q want public", v)
	}
}

// A wrong token on the WebSocket is refused with 401 (not downgraded to public).
func TestWSBadToken401(t *testing.T) {
	w := world.NewDefault()
	ts := httptest.NewServer(New(w, testTokens, "", nil).Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, resp, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/world?token=nope", nil)
	if err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad token: err=%v resp=%v", err, resp)
	}
}

// With no tokens configured nobody can act; reads stay public.
func TestNoTokensConfigured(t *testing.T) {
	var none Tokens
	if who, err := none.Who(""); who != "" || err != nil {
		t.Fatalf("empty token: %q %v", who, err)
	}
	if _, err := none.Who("anything"); err != ErrBadToken {
		t.Fatalf("want ErrBadToken, got %v", err)
	}
}

func TestLoadTokens(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "tokens.json")
	os.WriteFile(file, []byte(`{"nostarou":"aaa","labomi":"bbb"}`), 0o600)
	env := map[string]string{"CRAB_TOKENS_FILE": file, "CRAB_TOKENS": "guest:ccc"}
	tk, err := LoadTokens(func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	for tok, want := range map[string]string{"aaa": "nostarou", "bbb": "labomi", "ccc": "guest"} {
		if who, err := tk.Who(tok); err != nil || who != want {
			t.Errorf("Who(%q) = %q, %v; want %q", tok, who, err, want)
		}
	}
	for _, bad := range []map[string]string{
		{"CRAB_TOKENS": "nocolon"},
		{"CRAB_TOKENS": "a:same,b:same"},
		{"CRAB_TOKENS_FILE": filepath.Join(dir, "missing.json")},
	} {
		if _, err := LoadTokens(func(k string) string { return bad[k] }); err == nil {
			t.Errorf("LoadTokens(%v): want error", bad)
		}
	}
}
