package extgate

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Tick: every TickInterval crab-town tells the agent that time has passed --
// recent events, where it is, what it can see and the operations it can take.
// Whether to do anything is the agent's call; crab-town never moves it.
//
// A tick is held back while
//   - a guest is talking with the agent (a talk to it within TalkQuiet, or a
//     turn of the agent is running), or
//   - the agent has not finished answering the previous tick (its said was
//     admitted and no turn has ended since). TickStale is a safety valve: an
//     outstanding tick older than that is given up so ticks cannot stop forever.

// Env keys for the tick (seconds; 0 disables the tick).
const (
	EnvTickSeconds      = "CRAB_EXTGATE_TICK_SECONDS"
	EnvTalkQuietSeconds = "CRAB_EXTGATE_TALK_QUIET_SECONDS"
)

// Defaults: 10 minutes between ticks, 5 minutes of quiet after a talk.
const (
	DefaultTickInterval = 600 * time.Second
	DefaultTalkQuiet    = 300 * time.Second
	recentMax           = 5
)

// TickConfig reads the tick timings from env (defaults when unset).
func TickConfig(getenv func(string) string) (interval, quiet time.Duration, err error) {
	interval, quiet = DefaultTickInterval, DefaultTalkQuiet
	parse := func(key string, dst *time.Duration) {
		if v := getenv(key); v != "" && err == nil {
			n, e := strconv.ParseUint(v, 10, 32)
			if e != nil {
				err = fmt.Errorf("%s: %w", key, e)
				return
			}
			*dst = time.Duration(n) * time.Second
		}
	}
	parse(EnvTickSeconds, &interval)
	parse(EnvTalkQuietSeconds, &quiet)
	return interval, quiet, err
}

// tickState is what the bridge remembers between ticks.
type tickState struct {
	mu          sync.Mutex
	recent      []string // last few events, oldest first ("15:04 what")
	lastTalk    time.Time
	active      map[string]bool // activity ids started and not ended
	outstanding bool            // a tick was admitted and no turn has ended since
	sentAt      time.Time
}

func (b *Bridge) clock() time.Time {
	if b.Now != nil {
		return b.Now()
	}
	return time.Now()
}

// noteEvent records an event the agent was told about (for "最近の出来事").
func (b *Bridge) noteEvent(what string, talk bool) {
	t := &b.tick
	now := b.clock()
	t.mu.Lock()
	defer t.mu.Unlock()
	one := strings.ReplaceAll(what, "\n", " / ")
	if r := []rune(one); len(r) > 200 {
		one = string(r[:200]) + "…"
	}
	line := now.Format("15:04") + " " + one
	t.recent = append(t.recent, line)
	if len(t.recent) > recentMax {
		t.recent = t.recent[len(t.recent)-recentMax:]
	}
	if talk {
		t.lastTalk = now
	}
}

// noteActivity tracks turns: started / ended (other states are notices only).
func (b *Bridge) noteActivity(id, state string) {
	t := &b.tick
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.active == nil {
		t.active = map[string]bool{}
	}
	switch state {
	case "started":
		t.active[id] = true
	case "ended":
		delete(t.active, id)
		if len(t.active) == 0 {
			t.outstanding = false // the agent has finished answering
		}
	}
}

// tickBlocked reports why a tick must not be sent now ("" = may send).
func (b *Bridge) tickBlocked() string {
	t := &b.tick
	now := b.clock()
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.outstanding {
		if b.TickStale <= 0 || now.Sub(t.sentAt) < b.TickStale {
			return "previous tick not answered"
		}
		log.Printf("extgate: tick: previous tick unanswered for %s, giving up on it", now.Sub(t.sentAt))
		t.outstanding = false
	}
	if len(t.active) > 0 {
		return "a turn is running"
	}
	if !t.lastTalk.IsZero() && now.Sub(t.lastTalk) < b.TalkQuiet {
		return "talk in progress"
	}
	return ""
}

// tickSaid builds the "time has passed" said.
func (b *Bridge) tickSaid() Said {
	t := &b.tick
	t.mu.Lock()
	recent := append([]string(nil), t.recent...)
	t.mu.Unlock()
	what := fmt.Sprintf("時間が経った（%s ごとの定期の知らせ、現在 %s）", b.TickInterval, b.clock().Format("15:04"))
	if len(recent) == 0 {
		what += "\n最近の出来事: なし"
	} else {
		what += "\n最近の出来事:\n  " + strings.Join(recent, "\n  ")
	}
	n := b.origin.Add(1)
	return Said{
		Origin:      fmt.Sprintf("crab-town:tick:%d:%d", b.clock().UnixNano(), n),
		Text:        b.describe(what),
		AuthorLabel: "crab-town",
	}
}

// TickOnce sends one tick unless held back. It reports whether it was sent
// and admitted by core.
func (b *Bridge) TickOnce(ctx context.Context) (bool, string) {
	if why := b.tickBlocked(); why != "" {
		return false, why
	}
	s := b.tickSaid()
	seq, err := b.Client.Said(ctx, s)
	if err != nil {
		return false, err.Error()
	}
	if seq == nil {
		return false, "not admitted"
	}
	t := &b.tick
	t.mu.Lock()
	t.outstanding, t.sentAt = true, b.clock()
	t.mu.Unlock()
	return true, ""
}

func (b *Bridge) tickLoop(ctx context.Context) {
	if b.TickInterval <= 0 {
		return
	}
	ticker := time.NewTicker(b.TickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if sent, why := b.TickOnce(ctx); sent {
				log.Printf("extgate: tick sent")
			} else {
				log.Printf("extgate: tick held: %s", why)
			}
		}
	}
}

// tickFromEnv applies env timings to b (used by NewBridge).
func (b *Bridge) tickFromEnv() {
	interval, quiet, err := TickConfig(os.Getenv)
	if err != nil {
		log.Printf("extgate: tick config: %v (using defaults)", err)
		interval, quiet = DefaultTickInterval, DefaultTalkQuiet
	}
	b.TickInterval, b.TalkQuiet, b.TickStale = interval, quiet, 3*interval
}
