package extgate

import (
	"fmt"
	"strings"

	"github.com/kojira/crab-town/internal/world"
)

// View renders what the agent's actor can see of its room, as a compact text
// table: where it stands, the visible zones, doors, furniture and actors.
// Everything comes from World.ViewSnapshot(actor), so zones the actor may not
// see (other people's private rooms, an occupied bath) never appear -- nor
// does anything inside them. Facts only; nothing tells the agent what to do.
func (b *Bridge) View() string {
	actors, body := b.viewParts()
	if actors == "" {
		return body
	}
	// title, 現在地, then the people, then the rest
	l := strings.SplitN(body, "\n", 3)
	if len(l) < 3 {
		return body + "\n" + actors
	}
	return l[0] + "\n" + l[1] + "\n" + actors + "\n" + l[2]
}

// viewParts renders the map as the line of visible actors and the rest (title,
// position, zones, doors, furniture). describe puts the actors line near the
// top of a said and the long rest last, after the operations: core passes on
// only the first part of a long said. actors is "" when the map is unknown.
func (b *Bridge) viewParts() (actors, body string) {
	s := b.World.ViewSnapshot(b.Actor)
	var me *world.Actor
	for _, a := range s.Actors {
		if a.ID == b.Actor {
			me = a
		}
	}
	if me == nil {
		return "", "地図: 不明（あなたのアクターが町にいない）"
	}
	var r *world.Room
	for _, rm := range s.Rooms {
		if rm.ID == me.RoomID {
			r = rm
		}
	}
	if r == nil {
		return "", "地図: 不明"
	}
	hidden := map[string]bool{}
	for _, id := range r.HiddenZones {
		hidden[id] = true
	}
	// a tile is visible if its zone is; a tile between zones (door) only if
	// every zone touching it is (same rule as the world's CanSeeTile)
	visible := func(p world.Pos) bool {
		if z := r.ZoneAt(p); z != nil {
			return !hidden[z.ID]
		}
		for _, d := range []world.Pos{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
			if z := r.ZoneAt(world.Pos{X: p.X + d.X, Y: p.Y + d.Y}); z != nil && hidden[z.ID] {
				return false
			}
		}
		return true
	}
	pos := func(p world.Pos) string { return fmt.Sprintf("(%d,%d)", p.X, p.Y) }

	var sb strings.Builder
	fmt.Fprintf(&sb, "地図（%s から見える範囲。町は %dx%d、左上 (0,0)、x 右へ・y 下へ）\n", me.Name, r.Width, r.Height)
	here := "(区画の境目)"
	if z := r.ZoneAt(me.Pos); z != nil {
		here = z.Name
	}
	fmt.Fprintf(&sb, "現在地: %s %s\n", pos(me.Pos), here)
	sb.WriteString("ゾーン（名前 x範囲 y範囲）:")
	for _, z := range r.Zones {
		if hidden[z.ID] {
			continue
		}
		name := z.Name
		if z.House != "" {
			name = z.House + "/" + name
		}
		fmt.Fprintf(&sb, "\n  %s x%d-%d y%d-%d", name, z.Rect.X, z.Rect.X+z.Rect.W-1, z.Rect.Y, z.Rect.Y+z.Rect.H-1)
	}

	var doors []string
	for _, d := range r.Doors {
		if visible(d) {
			doors = append(doors, pos(d))
		}
	}
	sb.WriteString("\nドア: " + strings.Join(doors, " "))

	// furniture grouped per zone, one line each: label[kind](x,y)
	byZone := map[string][]string{}
	var order []string
	for _, f := range r.Furniture {
		if !visible(f.Pos) {
			continue
		}
		key := "(その他)"
		if z := r.ZoneAt(f.Pos); z != nil {
			key = z.Name
			if z.House != "" {
				key = z.House + "/" + key
			}
		}
		if _, ok := byZone[key]; !ok {
			order = append(order, key)
		}
		item := f.Label + "[" + f.Kind + "]" + pos(f.Pos)
		if f.Function != "" {
			item += "{id=" + f.ID + ",立つ位置" + pos(f.Access) + "}"
		}
		byZone[key] = append(byZone[key], item)
	}
	sb.WriteString("\n家具（名前[kind](x,y)、使えるものは {id,立つ位置}）:")
	for _, k := range order {
		sb.WriteString("\n  " + k + ": " + strings.Join(byZone[k], " "))
	}

	return actorsLine(s, me), sb.String()
}
