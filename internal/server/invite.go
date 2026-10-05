package server

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kojira/crab-town/internal/world"
)

// LegacyHouse is the house a legacy CRAB_INVITED ("id,id" without house=)
// applies to: it used to be the only house.
const LegacyHouse = world.NostarouHouse

// ParseInvited parses CRAB_INVITED into house id -> invited actor ids.
//
//	"house=id,id;house2=id"  per house (house = house id or owner id)
//	"id,id"                  legacy: all ids go to LegacyHouse
//
// Blank ids are dropped. A house listed twice is an error.
func ParseInvited(s string) (map[string][]string, error) {
	out := map[string][]string{}
	for _, part := range strings.Split(s, ";") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		house, list := LegacyHouse, part
		if k, v, ok := strings.Cut(part, "="); ok {
			house, list = strings.TrimSpace(k), v
			if house == "" {
				return nil, fmt.Errorf("empty house in %q", part)
			}
		}
		if _, dup := out[house]; dup {
			return nil, fmt.Errorf("house %q listed twice", house)
		}
		ids := []string{}
		for _, id := range strings.Split(list, ",") {
			if id = strings.TrimSpace(id); id != "" {
				ids = append(ids, id)
			}
		}
		out[house] = ids
	}
	return out, nil
}

// ApplyInvited registers the invited ids from CRAB_INVITED on each house.
// Without this no house has invited ids, so even a valid guest token sees the
// invited zones as hidden. Returns what was applied and the ids that have no
// token (they can never view as invited).
func ApplyInvited(w *world.World, tokens Tokens, getenv func(string) string) (applied map[string][]string, noToken []string, err error) {
	applied, err = ParseInvited(getenv("CRAB_INVITED"))
	if err != nil {
		return nil, nil, fmt.Errorf("CRAB_INVITED: %w", err)
	}
	houses := make([]string, 0, len(applied))
	for h := range applied {
		houses = append(houses, h)
	}
	sort.Strings(houses)
	seen := map[string]bool{}
	for _, h := range houses {
		if err := w.SetInvited(h, applied[h]); err != nil {
			return nil, nil, fmt.Errorf("CRAB_INVITED %s: %w", h, err)
		}
		for _, id := range applied[h] {
			if _, ok := tokens[id]; !ok && !seen[id] {
				seen[id] = true
				noToken = append(noToken, id)
			}
		}
	}
	return applied, noToken, nil
}
