package server

import (
	"fmt"
	"strings"

	"github.com/kojira/crab-town/internal/world"
)

// DefaultRoom is the room CRAB_INVITED applies to (the only house for now).
const DefaultRoom = "nostarou-room"

// ParseInvited parses CRAB_INVITED="id,id" into actor ids (blanks dropped).
func ParseInvited(s string) []string {
	out := []string{}
	for _, id := range strings.Split(s, ",") {
		if id = strings.TrimSpace(id); id != "" {
			out = append(out, id)
		}
	}
	return out
}

// ApplyInvited registers the invited ids from CRAB_INVITED on DefaultRoom.
// Without this the room has no invited ids, so even a valid guest token sees
// the invited zones (hobby, guest) as hidden. Returns the ids applied and the
// ones that have no token (they can never view as invited).
func ApplyInvited(w *world.World, tokens Tokens, getenv func(string) string) (ids, noToken []string, err error) {
	ids = ParseInvited(getenv("CRAB_INVITED"))
	if err := w.SetInvited(DefaultRoom, ids); err != nil {
		return nil, nil, fmt.Errorf("CRAB_INVITED: %w", err)
	}
	for _, id := range ids {
		if _, ok := tokens[id]; !ok {
			noToken = append(noToken, id)
		}
	}
	return ids, noToken, nil
}
