package extgate

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync/atomic"

	"github.com/kojira/crab-town/internal/world"
)

// Bridge maps extgate traffic onto the world, without deciding anything for
// the agent:
//   - world events (interact / knock / talk) -> said to core: what happened,
//     where the actor is, and the operations crab-town accepts right now
//   - say -> a speech bubble over the actor (also published as public state)
//   - activity (turn started / ended) is only a notice; it moves nothing
type Bridge struct {
	World  *world.World
	Actor  string // the agent's actor in the world
	Client *Client

	origin atomic.Uint64
}

// NewBridge wires cfg to w. Call Run to connect.
func NewBridge(w *world.World, cfg Config) *Bridge {
	b := &Bridge{World: w, Actor: cfg.Actor}
	b.Client = New(cfg, b)
	return b
}

// Run connects to core and forwards world events until ctx ends.
func (b *Bridge) Run(ctx context.Context) {
	events, cancel := b.World.Subscribe()
	defer cancel()
	go b.Client.Run(ctx)
	queue := make(chan Said, 32)
	go b.sendLoop(ctx, queue)
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			s, ok := b.toSaid(ev)
			if !ok {
				continue
			}
			select {
			case queue <- s:
			default:
				log.Printf("extgate: said queue full, dropped %s", s.Origin)
			}
		}
	}
}

func (b *Bridge) sendLoop(ctx context.Context, queue <-chan Said) {
	for {
		select {
		case <-ctx.Done():
			return
		case s := <-queue:
			seq, err := b.Client.Said(ctx, s)
			switch {
			case err != nil:
				log.Printf("extgate: said %s: %v", s.Origin, err)
			case seq == nil:
				log.Printf("extgate: said %s: not admitted", s.Origin)
			default:
				log.Printf("extgate: said %s: seq=%d", s.Origin, *seq)
			}
		}
	}
}

// toSaid turns an interact / knock / talk world event into a said whose text
// reports what happened and what the agent can do. It never says what to do.
func (b *Bridge) toSaid(ev world.Event) (Said, bool) {
	var what string
	switch ev.Type {
	case "interact":
		if ev.Furniture == nil || ev.Actor == nil {
			return Said{}, false
		}
		what = fmt.Sprintf("%s が %s で %s（%s）を使った",
			ev.Actor.Name, b.place(ev.Actor.ID, ev.Room), ev.Furniture.Label, ev.Furniture.Function)
		if ev.By != "" && ev.By != ev.Actor.ID {
			what += "（操作: " + ev.By + "）"
		}
	case "knock":
		door := ev.House
		if door == "" {
			door = ev.Room
		}
		what = fmt.Sprintf("%s が %s のドアをノックした", ev.By, door)
		if m := strings.TrimSpace(ev.Message); m != "" {
			what += "\n本文: " + m
		}
	case world.EventTalk:
		if ev.To != b.Actor {
			return Said{}, false
		}
		who := ev.By
		if ev.Role != "" {
			who += "（" + ev.Role + "）"
		}
		to := ev.To
		if wa, ok := b.World.Where(ev.To); ok {
			to = wa.Actor
		}
		what = fmt.Sprintf("%s が %s に話しかけた（Nostr 経由、平文）\n本文: %s", who, to, ev.Message)
	default:
		return Said{}, false
	}
	n := b.origin.Add(1)
	origin := fmt.Sprintf("crab-town:%s:%d:%d", ev.Type, ev.Time.UnixNano(), n)
	return Said{Origin: origin, Text: b.describe(what), AuthorLabel: label(ev.By)}, true
}

// place is where id stands if it is an actor in the world, else the room.
func (b *Bridge) place(id, room string) string {
	if wa, ok := b.World.Where(id); ok {
		return wa.String()
	}
	if room == "" {
		return "(場所不明)"
	}
	return room
}

// describe frames one event: what happened, where the agent's actor is now,
// and the operations crab-town accepts (generated from the dispatch table).
func (b *Bridge) describe(what string) string {
	var sb strings.Builder
	sb.WriteString("[crab-town] 出来事: ")
	sb.WriteString(what)
	sb.WriteString("\n")
	if wa, ok := b.World.Where(b.Actor); ok {
		sb.WriteString("あなた（" + wa.Actor + "）の現在地: " + wa.String() + "\n")
	} else {
		sb.WriteString("あなたの現在地: 不明\n")
	}
	sb.WriteString("crab-town で今取れる操作:")
	for _, op := range AgentOperations() {
		sb.WriteString("\n- " + op.Name + ": " + op.Desc)
	}
	return sb.String()
}

// label makes an author_label core accepts (nonblank, <=100 chars, no control chars).
func label(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
	if r := []rune(s); len(r) > 100 {
		s = string(r[:100])
	}
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return s
}

// OnSay implements Handler: show it as a bubble over the actor.
func (b *Bridge) OnSay(_, _, text string) error {
	return b.World.Speak(b.Actor, text)
}

// OnActivity implements Handler. A turn starting or ending is a notice from
// core, not something the agent chose: the world does not react to it.
func (b *Bridge) OnActivity(_, _, _, _ string) {}

// OnDisconnect implements Handler. Nothing in the world depends on the link.
func (b *Bridge) OnDisconnect() {}
