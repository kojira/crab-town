package nostr

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/kojira/crab-town/internal/world"
)

var (
	ErrKind      = errors.New("not a command event")
	ErrNotForUs  = errors.New("command is not addressed to this town")
	ErrStale     = errors.New("created_at outside the accepted window")
	ErrReplay    = errors.New("duplicate event id (replay)")
	ErrGuestOnly = errors.New("forbidden: guests may only knock or talk")
	ErrUnknown   = errors.New("unknown command")
)

// Roles a verified sender can have.
const (
	RoleOwner = "owner"
	RoleGuest = "guest"
)

// Command is the JSON content of a KindCommand event.
type Command struct {
	Type    string `json:"type"`              // move | snapshot | knock | talk
	Actor   string `json:"actor,omitempty"`   // move: actor id (default: the owner's actor)
	X       int    `json:"x,omitempty"`       // move
	Y       int    `json:"y,omitempty"`       // move
	Room    string `json:"room,omitempty"`    // knock: house id or room id
	Message string `json:"message,omitempty"` // knock
	Text    string `json:"text,omitempty"`    // talk (plain text, <= world.MaxTalk runes)
	To      string `json:"to,omitempty"`      // talk: actor id (default: Handler.TalkTo)
}

// Result is what the handler decided for one command event.
type Result struct {
	Cmd      string // command type ("" when the event was ignored before parsing)
	Role     string
	Err      error
	Snapshot bool // the sender asked for a snapshot
	Reply    bool // false = drop silently (forged, stale, replay, not for us)
}

// Handler turns verified command events into world calls. It is safe for
// concurrent use (several relays may deliver the same event at once).
type Handler struct {
	World      *world.World
	Town       string        // this town's pubkey (hex): commands must p-tag it
	Owner      string        // owner pubkey (hex): full rights
	OwnerActor string        // world id the owner acts as (e.g. "nostarou")
	TalkTo     string        // actor a talk goes to when "to" is omitted (e.g. "nostarou")
	Window     time.Duration // accepted |now - created_at|
	// NotBefore: events created before this (the process start) are dropped.
	// The replay set lives in memory, and some relays re-deliver recent
	// ephemeral events to a new subscription, so without it a restart would
	// re-apply commands that were already handled.
	NotBefore time.Time
	Now       func() time.Time

	mu   sync.Mutex
	seen map[string]int64 // event id -> created_at (kept for 2*Window)
}

func (h *Handler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

// Role reports the role of a (verified) pubkey.
func (h *Handler) Role(pubkey string) string {
	if h.Owner != "" && pubkey == h.Owner {
		return RoleOwner
	}
	return RoleGuest
}

// GuestID is the world id a guest pubkey acts as.
func GuestID(pubkey string) string {
	if len(pubkey) > 16 {
		pubkey = pubkey[:16]
	}
	return "nostr:" + pubkey
}

// admit runs every check that does not depend on the command itself: kind,
// addressing, signature, freshness and replay. Only an event passing all of
// them is remembered as seen, so forged copies cannot burn a real event's id.
func (h *Handler) admit(ev *Event) error {
	if ev.Kind != KindCommand {
		return ErrKind
	}
	if ev.Tag("p") != h.Town {
		return ErrNotForUs
	}
	if err := ev.Verify(); err != nil {
		return err
	}
	now := h.now().Unix()
	win := int64(h.Window / time.Second)
	if d := now - ev.CreatedAt; d > win || d < -win || ev.CreatedAt < h.NotBefore.Unix() {
		return ErrStale
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.seen == nil {
		h.seen = map[string]int64{}
	}
	for id, at := range h.seen {
		if now-at > 2*win {
			delete(h.seen, id)
		}
	}
	if _, dup := h.seen[ev.ID]; dup {
		return ErrReplay
	}
	h.seen[ev.ID] = ev.CreatedAt
	return nil
}

// Handle checks and applies one event. Events failing admission are dropped
// without a reply (Reply=false); admitted ones always get a result.
func (h *Handler) Handle(ev *Event) Result {
	if err := h.admit(ev); err != nil {
		return Result{Err: err}
	}
	res := Result{Role: h.Role(ev.PubKey), Reply: true}
	var cmd Command
	if err := json.Unmarshal([]byte(ev.Content), &cmd); err != nil {
		res.Err = world.ErrBadRequest
		return res
	}
	res.Cmd = cmd.Type
	switch cmd.Type {
	case "snapshot":
		// read only: the public view that is broadcast anyway
		res.Snapshot = true
	case "knock":
		by := GuestID(ev.PubKey)
		if res.Role == RoleOwner {
			by = h.OwnerActor
		}
		res.Err = h.World.Knock(by, cmd.Room, clip(cmd.Message, 280))
	case "talk":
		// anyone may talk; the sender is named by pubkey, the role says who it is
		to := cmd.To
		if to == "" {
			to = h.TalkTo
		}
		res.Err = h.World.Talk(GuestID(ev.PubKey), res.Role, to, cmd.Text)
	case "move":
		if res.Role != RoleOwner {
			res.Err = ErrGuestOnly
			break
		}
		actor := cmd.Actor
		if actor == "" {
			actor = h.OwnerActor
		}
		res.Err = h.World.Move(h.OwnerActor, actor, world.Pos{X: cmd.X, Y: cmd.Y})
	default:
		res.Err = ErrUnknown
	}
	return res
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
