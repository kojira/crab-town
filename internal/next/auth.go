package next

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/kojira/crab-town/internal/nostr"
)

// NIP-98 HTTP Auth: every request that acts carries
//
//	Authorization: Nostr <base64(kind 27235 event)>
//
// signed by the caller's own key, with tags u (the absolute URL), method, and
// payload (sha256 hex of the body) when there is a body. The pubkey that
// signed it is who the request is. Nothing else (no token, no "AI or human"
// flag) is looked at. Browsers sign with NIP-07; agents with their own key.
const KindHTTPAuth = 27235

var (
	ErrNoAuth      = errors.New("NIP-98 Authorization required")
	ErrAuthFormat  = errors.New("malformed NIP-98 Authorization")
	ErrAuthKind    = errors.New("NIP-98: wrong kind")
	ErrAuthURL     = errors.New("NIP-98: u tag does not match this URL")
	ErrAuthMethod  = errors.New("NIP-98: method tag does not match")
	ErrAuthPayload = errors.New("NIP-98: payload tag does not match the body")
	ErrAuthStale   = errors.New("NIP-98: created_at outside the accepted window")
	ErrAuthReplay  = errors.New("NIP-98: event already used")
)

// Verifier checks NIP-98 events. Safe for concurrent use.
type Verifier struct {
	BaseURL string        // public base URL, e.g. "http://127.0.0.1:8788" (u = BaseURL + path)
	Window  time.Duration // accepted |now - created_at|
	Now     func() time.Time

	mu   sync.Mutex
	seen map[string]int64 // event id -> created_at, kept for 2*Window
}

func (v *Verifier) now() time.Time {
	if v.Now != nil {
		return v.Now()
	}
	return time.Now()
}

// AuthEvent decodes the Authorization header value ("Nostr <base64>").
func AuthEvent(header string) (*nostr.Event, error) {
	if header == "" {
		return nil, ErrNoAuth
	}
	b64, ok := strings.CutPrefix(header, "Nostr ")
	if !ok {
		return nil, ErrAuthFormat
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return nil, ErrAuthFormat
	}
	var ev nostr.Event
	if err := json.Unmarshal(raw, &ev); err != nil {
		return nil, ErrAuthFormat
	}
	return &ev, nil
}

// Check verifies one request: header value, method, path and body. Returns
// the signer's pubkey (hex). Only an event passing every check is remembered,
// so a forged copy cannot burn a real event's id.
func (v *Verifier) Check(header, method, path string, body []byte) (string, error) {
	ev, err := AuthEvent(header)
	if err != nil {
		return "", err
	}
	if ev.Kind != KindHTTPAuth {
		return "", ErrAuthKind
	}
	if err := ev.Verify(); err != nil {
		return "", err
	}
	if ev.Tag("u") != strings.TrimRight(v.BaseURL, "/")+path {
		return "", ErrAuthURL
	}
	if !strings.EqualFold(ev.Tag("method"), method) {
		return "", ErrAuthMethod
	}
	if len(body) > 0 || ev.Tag("payload") != "" {
		sum := sha256.Sum256(body)
		if ev.Tag("payload") != hex.EncodeToString(sum[:]) {
			return "", ErrAuthPayload
		}
	}
	now := v.now().Unix()
	win := int64(v.Window / time.Second)
	if d := now - ev.CreatedAt; d > win || d < -win {
		return "", ErrAuthStale
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.seen == nil {
		v.seen = map[string]int64{}
	}
	for id, at := range v.seen {
		if now-at > 2*win {
			delete(v.seen, id)
		}
	}
	if _, dup := v.seen[ev.ID]; dup {
		return "", ErrAuthReplay
	}
	v.seen[ev.ID] = ev.CreatedAt
	return ev.PubKey, nil
}

// CheckRequest is Check for an *http.Request whose body was already read.
// A WebSocket handshake cannot carry headers from a browser, so ?auth= (the
// same "Nostr <base64>" value) is accepted for GET.
func (v *Verifier) CheckRequest(r *http.Request, body []byte) (string, error) {
	h := r.Header.Get("Authorization")
	if h == "" && r.Method == http.MethodGet {
		h = r.URL.Query().Get("auth")
	}
	return v.Check(h, r.Method, r.URL.Path, body)
}
