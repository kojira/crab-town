package world

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

// EventTalk: someone spoke to an actor (plain text, nothing is encrypted).
const EventTalk = "talk"

// MaxTalk caps what a visitor may say in one talk (runes). Longer is refused,
// not cut, so the sender knows it did not go through as written.
const MaxTalk = 280

var ErrTooLong = errors.New("text too long")

// Talk emits a talk event: by (with the role it was verified as) speaks to the
// actor to. Anyone with an id may talk; the world itself does nothing about it.
func (w *World) Talk(by, role, to, text string) error {
	text = CleanText(text)
	if by == "" || strings.TrimSpace(text) == "" {
		return ErrBadRequest
	}
	if utf8.RuneCountInString(text) > MaxTalk {
		return ErrTooLong
	}
	w.mu.Lock()
	a, r, err := w.lookup(to)
	if err != nil {
		w.mu.Unlock()
		return err
	}
	ev := Event{Type: EventTalk, Room: r.ID, By: by, Role: role, To: a.ID, Message: text}
	w.mu.Unlock()
	w.emit(ev)
	return nil
}

// CleanText drops control characters (newlines are kept, CRLF becomes LF) and
// invalid UTF-8.
func CleanText(s string) string {
	s = strings.ToValidUTF8(strings.ReplaceAll(s, "\r\n", "\n"), "")
	return strings.Map(func(r rune) rune {
		if r == '\n' {
			return r
		}
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) {
			return -1
		}
		return r
	}, s)
}

// Whereabouts is where an actor is, in words a reader can follow.
type Whereabouts struct {
	Actor string // display name
	House string // house id, "" outdoors
	Zone  string // zone name, "" on a wall / door tile
	Pos   Pos
	Using string // label of the furniture in use, "" if none
	State string
}

// Where reports where the actor is right now.
func (w *World) Where(actorID string) (Whereabouts, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	a, r, err := w.lookup(actorID)
	if err != nil {
		return Whereabouts{}, false
	}
	wa := Whereabouts{Actor: a.Name, Pos: a.Pos, State: a.State}
	if h := r.HouseAt(a.Pos); h != nil {
		wa.House = h.ID
	}
	if z := r.ZoneAt(a.Pos); z != nil {
		wa.Zone = z.Name
	}
	if f := r.furniture(a.Using); f != nil {
		wa.Using = f.Label
	}
	return wa, true
}

// String: "nostarou-house / リビング (48,3), 状態 idle".
func (wa Whereabouts) String() string {
	var b strings.Builder
	place := wa.Zone
	if wa.House != "" {
		place = wa.House + " / " + place
	}
	if place == "" {
		place = "(区画の境目)"
	}
	b.WriteString(place)
	b.WriteString(" (" + strconv.Itoa(wa.Pos.X) + "," + strconv.Itoa(wa.Pos.Y) + ")、状態 " + wa.State)
	if wa.Using != "" {
		b.WriteString("、" + wa.Using + " を使用中")
	}
	return b.String()
}
