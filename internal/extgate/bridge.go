package extgate

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/kojira/crab-town/internal/world"
)

// WorkFurniture is where the actor goes while core runs a turn.
const WorkFurniture = "pc"

// Bridge maps extgate traffic onto the world:
//   - activity started → the actor walks to the study PC; ended → back to what it was doing
//   - say → a speech bubble (display only; nothing is posted anywhere)
//   - furniture interact / knock events in the world → said to core
type Bridge struct {
	World  *world.World
	Actor  string // actor driven by activity / say
	Client *Client

	mu     sync.Mutex
	active map[string]bool // activity ids currently started
	saved  *world.Actor    // actor before the first started
	self   map[string]int  // interacts caused by the bridge itself (furniture id → count)
	origin atomic.Uint64
}

// NewBridge wires cfg to w. Call Run to connect.
func NewBridge(w *world.World, cfg Config) *Bridge {
	b := &Bridge{World: w, Actor: cfg.Actor, active: map[string]bool{}, self: map[string]int{}}
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

// toSaid turns an interact / knock world event into a said, skipping
// interactions the bridge caused itself (walking to the PC).
func (b *Bridge) toSaid(ev world.Event) (Said, bool) {
	n := b.origin.Add(1)
	origin := fmt.Sprintf("crab-town:%s:%d:%d", ev.Type, ev.Time.UnixNano(), n)
	switch ev.Type {
	case "interact":
		if ev.Furniture == nil || ev.Actor == nil {
			return Said{}, false
		}
		b.mu.Lock()
		if ev.Actor.ID == b.Actor && b.self[ev.Furniture.ID] > 0 {
			b.self[ev.Furniture.ID]--
			b.mu.Unlock()
			return Said{}, false
		}
		b.mu.Unlock()
		text := fmt.Sprintf("[crab-town] %s が %s（%s）を使った（操作: %s）",
			ev.Actor.Name, ev.Furniture.Label, ev.Furniture.Function, ev.By)
		return Said{Origin: origin, Text: text, AuthorLabel: label(ev.By)}, true
	case "knock":
		text := fmt.Sprintf("[crab-town] %s が %s をノックした", ev.By, ev.Room)
		if m := strings.TrimSpace(ev.Message); m != "" {
			text += ": " + m
		}
		return Said{Origin: origin, Text: text, AuthorLabel: label(ev.By)}, true
	}
	return Said{}, false
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

// OnActivity implements Handler.
func (b *Bridge) OnActivity(_, _, activityID, state string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch state {
	case "started":
		if b.active[activityID] {
			return
		}
		b.active[activityID] = true
		if len(b.active) == 1 {
			b.startWorkLocked()
		}
	case "ended":
		if !b.active[activityID] {
			return
		}
		delete(b.active, activityID)
		if len(b.active) == 0 {
			b.restoreLocked()
		}
	}
}

// OnDisconnect implements Handler: no turn can be running without core.
func (b *Bridge) OnDisconnect() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.active) > 0 {
		b.active = map[string]bool{}
		b.restoreLocked()
	}
}

func (b *Bridge) startWorkLocked() {
	a, ok := b.World.Actor(b.Actor)
	if !ok {
		log.Printf("extgate: actor %q not found", b.Actor)
		return
	}
	b.saved = &a
	if a.Using == WorkFurniture {
		return // already at the PC
	}
	b.self[WorkFurniture]++
	if err := b.World.Interact(b.Actor, b.Actor, WorkFurniture); err != nil {
		b.self[WorkFurniture]--
		log.Printf("extgate: activity: walk to %s: %v", WorkFurniture, err)
	}
}

// restoreLocked returns the actor to what it was doing before the turn:
// back to the furniture it was using, or back to the tile it stood on.
func (b *Bridge) restoreLocked() {
	s := b.saved
	b.saved = nil
	if s == nil {
		return
	}
	var err error
	switch {
	case s.Using == WorkFurniture:
		return
	case s.Using != "":
		b.self[s.Using]++
		if err = b.World.Interact(b.Actor, b.Actor, s.Using); err != nil {
			b.self[s.Using]--
		}
	default:
		dest := s.Pos
		if s.Target != nil {
			dest = *s.Target
		}
		err = b.World.Move(b.Actor, b.Actor, dest)
	}
	if err != nil {
		log.Printf("extgate: activity: restore: %v", err)
	}
}
