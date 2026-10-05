package nostr

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/kojira/crab-town/internal/world"
)

// DefaultRelays are used when CRAB_NOSTR_RELAYS is unset. All of them forward
// ephemeral events (checked by hand, see README).
var DefaultRelays = []string{"wss://r.kojira.io", "wss://n.kojira.io", "wss://x.kojira.io"}

// Config is the Nostr bridge configuration (from CRAB_NOSTR_* env vars).
type Config struct {
	Relays     []string
	KeyFile    string        // crab-town's own key (0600, created on first run)
	Owner      string        // owner pubkey hex
	OwnerActor string        // world actor the owner acts as
	TalkTo     string        // actor a talk goes to by default (CRAB_NOSTR_TALK_TO)
	Window     time.Duration // accepted created_at skew
}

// LoadConfig reads CRAB_NOSTR_*. nil = disabled (CRAB_NOSTR_KEY_FILE unset).
func LoadConfig(getenv func(string) string) (*Config, error) {
	kf := getenv("CRAB_NOSTR_KEY_FILE")
	if kf == "" {
		return nil, nil
	}
	c := &Config{KeyFile: kf, Relays: DefaultRelays, OwnerActor: "nostarou", TalkTo: "nostarou", Window: 2 * time.Minute}
	if v := getenv("CRAB_NOSTR_RELAYS"); v != "" {
		c.Relays = nil
		for _, r := range strings.Split(v, ",") {
			if r = strings.TrimSpace(r); r != "" {
				if !strings.HasPrefix(r, "wss://") && !strings.HasPrefix(r, "ws://") {
					return nil, fmt.Errorf("CRAB_NOSTR_RELAYS: %q is not a ws(s) url", r)
				}
				c.Relays = append(c.Relays, r)
			}
		}
	}
	if len(c.Relays) == 0 {
		return nil, fmt.Errorf("CRAB_NOSTR_RELAYS: no relay")
	}
	if v := getenv("CRAB_NOSTR_OWNER"); v != "" {
		pk, err := ParsePubKey(v)
		if err != nil {
			return nil, fmt.Errorf("CRAB_NOSTR_OWNER: %w", err)
		}
		c.Owner = pk
	}
	if v := getenv("CRAB_NOSTR_OWNER_ACTOR"); v != "" {
		c.OwnerActor = v
	}
	if v := getenv("CRAB_NOSTR_TALK_TO"); v != "" {
		c.TalkTo = v
	}
	if v := getenv("CRAB_NOSTR_WINDOW"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return nil, fmt.Errorf("CRAB_NOSTR_WINDOW: invalid duration")
		}
		c.Window = d
	}
	return c, nil
}

// Bridge connects the world to Nostr relays.
type Bridge struct {
	cfg     Config
	key     *btcec.PrivateKey
	world   *world.World
	handler *Handler
	relays  []*relayConn

	mu       sync.Mutex
	lastSnap time.Time
}

// NewBridge loads (or creates) the town key and prepares the relays.
// It returns the town pubkey (hex) for logging; the secret never leaves.
func NewBridge(w *world.World, cfg Config) (*Bridge, string, error) {
	key, created, err := LoadOrCreateKey(cfg.KeyFile)
	if err != nil {
		return nil, "", err
	}
	if created {
		log.Printf("nostr: created a new town key file (0600)")
	}
	town := PubHex(key)
	b := &Bridge{cfg: cfg, key: key, world: w}
	b.handler = &Handler{World: w, Town: town, Owner: cfg.Owner, OwnerActor: cfg.OwnerActor, TalkTo: cfg.TalkTo, Window: cfg.Window, NotBefore: time.Now()}
	for _, u := range cfg.Relays {
		rc := &relayConn{url: u, onEvent: b.onCommand}
		rc.filter = func() map[string]any {
			return map[string]any{
				"kinds": []int{KindCommand}, "#p": []string{town},
				"since": b.handler.NotBefore.Unix(),
			}
		}
		b.relays = append(b.relays, rc)
	}
	return b, town, nil
}

// Run connects to every relay and forwards world events until ctx ends.
func (b *Bridge) Run(ctx context.Context) {
	for _, rc := range b.relays {
		go rc.run(ctx)
	}
	events, cancel := b.world.Subscribe()
	defer cancel()
	tick := time.NewTicker(30 * time.Second) // keep late joiners in sync
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			b.publishSnapshot(ctx, true)
		case ev, ok := <-events:
			if !ok {
				return
			}
			// Nothing is encrypted: publish only what the public may see.
			if ev, ok = b.world.FilterEvent("", ev); ok {
				b.publish(ctx, ev, replyTags(ev))
			}
		}
	}
}

func (b *Bridge) onCommand(ev *Event) {
	res := b.handler.Handle(ev)
	if !res.Reply {
		if res.Err != ErrReplay && res.Err != ErrNotForUs {
			log.Printf("nostr: dropped event %.12s: %v", ev.ID, res.Err)
		}
		return
	}
	log.Printf("nostr: %s %s from %.12s: err=%v", res.Role, res.Cmd, ev.PubKey, res.Err)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out := map[string]any{"type": "result", "cmd": res.Cmd, "role": res.Role, "ok": res.Err == nil}
	if res.Err != nil {
		out["error"] = res.Err.Error()
	}
	b.publish(ctx, out, [][]string{{"e", ev.ID}, {"p", ev.PubKey}})
	if res.Snapshot {
		b.publishSnapshot(ctx, false)
	}
}

// publishSnapshot sends the public snapshot; requests are coalesced to one a
// second (everyone subscribed receives the same broadcast).
func (b *Bridge) publishSnapshot(ctx context.Context, force bool) {
	b.mu.Lock()
	if !force && time.Since(b.lastSnap) < time.Second {
		b.mu.Unlock()
		return
	}
	b.lastSnap = time.Now()
	b.mu.Unlock()
	b.publish(ctx, b.world.ViewSnapshot(""), nil)
}

func (b *Bridge) publish(ctx context.Context, content any, tags [][]string) {
	body, err := json.Marshal(content)
	if err != nil {
		return
	}
	ev := &Event{CreatedAt: time.Now().Unix(), Kind: KindState, Tags: append([][]string{{"t", "crab-town"}}, tags...), Content: string(body)}
	if err := ev.Sign(b.key); err != nil {
		log.Printf("nostr: sign: %v", err)
		return
	}
	for _, rc := range b.relays {
		rc.send(ctx, []any{"EVENT", ev}) // best effort; disconnected relays skip
	}
}

// replyTags marks a say that answers a talk: ["reply_to", <talker id>] (the
// same id as the talk's "by"), so a viewer can show "→ あなた".
func replyTags(ev world.Event) [][]string {
	if ev.Type != world.EventSay || ev.ReplyTo == "" {
		return nil
	}
	return [][]string{{"reply_to", ev.ReplyTo}}
}
