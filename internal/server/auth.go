package server

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// Tokens maps actor id -> secret token. A request proves it acts as an actor by
// presenting that actor's token. No token = public (anonymous) viewer.
type Tokens map[string]string

// ErrBadToken: a token was presented but matches no actor.
var ErrBadToken = errors.New("invalid token")

// TokenParam is the /world query parameter carrying a token (browsers cannot set
// headers on a WebSocket handshake). "Authorization: Bearer <token>" works too.
const TokenParam = "token"

// Who resolves a token to an actor id. ("", nil) = no token (public).
// Every entry is compared in constant time.
func (t Tokens) Who(token string) (string, error) {
	if token == "" {
		return "", nil
	}
	who := ""
	for id, secret := range t {
		if secret != "" && subtle.ConstantTimeCompare([]byte(secret), []byte(token)) == 1 {
			who = id
		}
	}
	if who == "" {
		return "", ErrBadToken
	}
	return who, nil
}

// requestToken reads "Authorization: Bearer <token>", falling back to ?token=.
func requestToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		tok, ok := strings.CutPrefix(h, "Bearer ")
		if !ok {
			return h // malformed header: still a (bad) token, so it is rejected, not ignored
		}
		return strings.TrimSpace(tok)
	}
	return r.URL.Query().Get(TokenParam)
}

// LoadTokens reads tokens from the environment (never from the repository):
//   - CRAB_TOKENS="id:token,id:token"
//   - CRAB_TOKENS_FILE=path to a JSON object {"id":"token"} kept out of git
//
// Both may be set; entries are merged (CRAB_TOKENS wins). Nothing set = no tokens.
func LoadTokens(getenv func(string) string) (Tokens, error) {
	t := Tokens{}
	if path := getenv("CRAB_TOKENS_FILE"); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("CRAB_TOKENS_FILE: %w", err)
		}
		if err := json.Unmarshal(b, &t); err != nil {
			return nil, fmt.Errorf("CRAB_TOKENS_FILE: %w", err)
		}
	}
	for _, kv := range strings.Split(getenv("CRAB_TOKENS"), ",") {
		if kv = strings.TrimSpace(kv); kv == "" {
			continue
		}
		id, tok, ok := strings.Cut(kv, ":")
		if !ok || id == "" || tok == "" {
			return nil, errors.New("CRAB_TOKENS: want id:token[,id:token]")
		}
		t[id] = tok
	}
	seen := map[string]string{}
	for id, tok := range t {
		if tok == "" {
			return nil, fmt.Errorf("empty token for %q", id)
		}
		if other, dup := seen[tok]; dup {
			return nil, fmt.Errorf("actors %q and %q share a token", other, id)
		}
		seen[tok] = id
	}
	return t, nil
}
