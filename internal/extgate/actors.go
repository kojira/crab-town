package extgate

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/kojira/crab-town/internal/world"
)

// actorKind names who an actor is: a resident of the town, the logged-in
// owner, or a guest.
func actorKind(a *world.Actor) string {
	switch a.Role {
	case world.RoleOwner:
		return "オーナー"
	case world.RoleGuest:
		return "来客"
	default:
		return "住人"
	}
}

// actorsLine lists the other actors me can see in its room, from a
// ViewSnapshot taken for me: anyone standing where me may not see is redacted
// (Hidden) there and left out here. "名前[種別](x,y)" each, sorted.
func actorsLine(s world.Snapshot, me *world.Actor) string {
	var out []string
	for _, a := range s.Actors {
		if a.RoomID != me.RoomID || a.Hidden || a.ID == me.ID {
			continue
		}
		out = append(out, fmt.Sprintf("%s[%s](%d,%d)", a.Name, actorKind(a), a.Pos.X, a.Pos.Y))
	}
	slices.Sort(out)
	if len(out) == 0 {
		out = []string{"なし"}
	}
	return "見える他のアクター（名前[種別](x,y)）: " + strings.Join(out, " ")
}

// visitorTracker remembers which zone each visitor was last seen in, so a
// visitor walking tile by tile is reported once per zone, not once per step.
type visitorTracker struct {
	mu   sync.Mutex
	zone map[string]string // visitor id -> "house/zone" last seen
}

// visitorSaid reports a visitor (owner / guest avatar) stepping into the zone
// the agent's actor is in. Only what the agent may see: the event goes
// through the world's visibility filter for the agent's actor first.
func (b *Bridge) visitorSaid(ev world.Event) (string, bool) {
	if ev.Actor == nil || ev.Actor.Role == "" || ev.Actor.ID == b.Actor {
		return "", false
	}
	seen, ok := b.World.FilterEvent(b.Actor, ev)
	if !ok || seen.Actor == nil || seen.Actor.Hidden {
		return "", false
	}
	a := seen.Actor
	h, z := b.World.PlaceAt(a.RoomID, a.Pos)
	if z == "" {
		return "", false // a doorway: not in any zone yet
	}
	where := h + "/" + z
	t := &b.visitors
	t.mu.Lock()
	if t.zone == nil {
		t.zone = map[string]string{}
	}
	changed := t.zone[a.ID] != where
	t.zone[a.ID] = where
	t.mu.Unlock()
	if !changed {
		return "", false
	}
	me, ok := b.World.Actor(b.Actor)
	if !ok || me.RoomID != a.RoomID {
		return "", false
	}
	if mh, mz := b.World.PlaceAt(me.RoomID, me.Pos); mh != h || mz != z {
		return "", false
	}
	place := z
	if h != "" {
		place = h + " / " + z
	}
	return fmt.Sprintf("%s（%s、%s）が、あなたのいる %s に入ってきた。今 (%d,%d) にいる",
		a.Name, actorKind(a), a.ID, place, a.Pos.X, a.Pos.Y), true
}
