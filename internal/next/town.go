package next

import (
	"errors"
	"slices"
	"sort"
	"sync"

	"github.com/kojira/crab-town/internal/world"
)

var (
	ErrNotVacant     = errors.New("plot is not vacant")
	ErrNoApplication = errors.New("no pending application for this plot and pubkey")
	ErrNotHolder     = errors.New("only the holder of this house may do that")
	ErrNotOwner      = errors.New("only the town owner may do that")
	ErrFull          = errors.New("the town is full")
	ErrNotJoined     = errors.New("join first")
)

// Town is the next-generation town: one world plus its data file. Every
// method takes the caller's pubkey (already verified by NIP-98) and nothing
// else about the caller -- there is no "kind of caller". The actor id of a
// pubkey is the pubkey itself.
type Town struct {
	World *world.World
	Path  string // data file ("" = not persisted, tests)

	mu   sync.Mutex // guards data and serialises join / plot changes
	data *Data
}

// New builds the world from data. Nobody is in the town until they join.
func New(d *Data, path string) (*Town, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	w := world.New()
	for _, r := range d.Rooms {
		rc := *r
		rc.Houses = nil
		for _, h := range r.Houses {
			hc := *h
			hc.Invited = append([]string{}, h.Invited...)
			rc.Houses = append(rc.Houses, &hc)
		}
		w.AddRoom(&rc)
	}
	return &Town{World: w, Path: path, data: d}, nil
}

func (t *Town) saveLocked() error {
	if t.Path == "" {
		return nil
	}
	return t.data.Save(t.Path)
}

func short(pk string) string { return pk[:8] }

// Join puts the caller's actor in the town (idempotent). Any verified pubkey
// may join; a known one appears with its name at its spot, others at the spawn.
func (t *Town) Join(pubkey string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.World.Actor(pubkey); ok {
		return nil
	}
	max := t.data.MaxActors
	if max <= 0 {
		max = DefaultMaxActors
	}
	if t.World.ActorCount() >= max {
		return ErrFull
	}
	a := world.Actor{ID: pubkey, Pubkey: pubkey, Name: short(pubkey), RoomID: t.data.Spawn.Room, Pos: t.data.Spawn.Pos}
	for _, n := range t.data.Names {
		if n.Pubkey == pubkey {
			a.Name = n.Name
			if n.Room != "" {
				a.RoomID, a.Pos = n.Room, n.Pos
			}
		}
	}
	_, err := t.World.Appear(a)
	return err
}

// Leave takes the caller's actor out.
func (t *Town) Leave(pubkey string) error { return t.World.Remove(pubkey) }

func (t *Town) joined(pubkey string) error {
	if _, ok := t.World.Actor(pubkey); !ok {
		return ErrNotJoined
	}
	return nil
}

// Move walks the caller's own actor. Same call for everyone.
func (t *Town) Move(pubkey string, to world.Pos) error {
	if err := t.joined(pubkey); err != nil {
		return err
	}
	return t.World.MoveExclusive(pubkey, pubkey, to)
}

// Interact walks the caller's own actor to a piece of furniture and uses it.
func (t *Town) Interact(pubkey, furniture string) error {
	if err := t.joined(pubkey); err != nil {
		return err
	}
	return t.World.InteractExclusive(pubkey, pubkey, furniture)
}

// Say shows a speech bubble over the caller's own actor.
func (t *Town) Say(pubkey, text string) error {
	if err := t.joined(pubkey); err != nil {
		return err
	}
	text = world.CleanText(text)
	if text == "" {
		return world.ErrBadRequest
	}
	if len([]rune(text)) > world.MaxTalk {
		return world.ErrTooLong
	}
	return t.World.Speak(pubkey, text)
}

// Knock on a house (any joined actor).
func (t *Town) Knock(pubkey, house, message string) error {
	if err := t.joined(pubkey); err != nil {
		return err
	}
	return t.World.Knock(pubkey, house, world.CleanText(message))
}

// Apply asks for a vacant plot. Anyone may apply (one pending application
// per pubkey and plot). It assigns nothing: the town owner approves.
func (t *Town) Apply(pubkey, house string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	h := t.data.house(house)
	if h == nil {
		return world.ErrNoHouse
	}
	if h.Owner != "" {
		return ErrNotVacant
	}
	app := Application{House: house, Pubkey: pubkey}
	if slices.Contains(t.data.Applications, app) {
		return nil
	}
	t.data.Applications = append(t.data.Applications, app)
	return t.saveLocked()
}

// Approve gives a vacant plot to an applicant (town owner only). The other
// applications for that plot are dropped.
func (t *Town) Approve(by, house, applicant string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if by != t.data.Owner {
		return ErrNotOwner
	}
	h := t.data.house(house)
	if h == nil {
		return world.ErrNoHouse
	}
	if h.Owner != "" {
		return ErrNotVacant
	}
	if !slices.Contains(t.data.Applications, Application{House: house, Pubkey: applicant}) {
		return ErrNoApplication
	}
	if err := t.World.SetHolder(house, applicant); err != nil {
		return err
	}
	h.Owner = applicant
	t.data.Applications = slices.DeleteFunc(t.data.Applications, func(a Application) bool { return a.House == house })
	return t.saveLocked()
}

// Release makes a house vacant again (its holder or the town owner).
func (t *Town) Release(by, house string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	h := t.data.house(house)
	if h == nil {
		return world.ErrNoHouse
	}
	if by != h.Owner && by != t.data.Owner {
		return ErrNotHolder
	}
	if err := t.World.SetHolder(house, ""); err != nil {
		return err
	}
	if err := t.World.SetInvited(house, nil); err != nil {
		return err
	}
	h.Owner, h.Invited = "", []string{}
	return t.saveLocked()
}

// SetInvited replaces who is invited to a house (its holder only).
func (t *Town) SetInvited(by, house string, invited []string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	h := t.data.house(house)
	if h == nil {
		return world.ErrNoHouse
	}
	if h.Owner == "" || by != h.Owner {
		return ErrNotHolder
	}
	for _, id := range invited {
		if !isPubHex(id) {
			return world.ErrBadRequest
		}
	}
	if err := t.World.SetInvited(house, invited); err != nil {
		return err
	}
	h.Invited = append([]string{}, invited...)
	return t.saveLocked()
}

// Rights is what a pubkey may do -- the only thing the town distinguishes.
type Rights struct {
	TownOwner bool     `json:"town_owner,omitempty"`
	Holds     []string `json:"holds,omitempty"`   // houses it holds
	Invited   []string `json:"invited,omitempty"` // houses it is invited to
	Label     string   `json:"label,omitempty"`   // display label derived from the above
}

// label: the strongest right, for display. Never "AI" / "human" / "guest".
func (r Rights) label() string {
	switch {
	case r.TownOwner:
		return "オーナー"
	case len(r.Holds) > 0:
		return "家主"
	case len(r.Invited) > 0:
		return "招待"
	}
	return ""
}

// RightsOf derives a pubkey's rights from the data.
func (t *Town) RightsOf(pubkey string) Rights {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.rightsLocked(pubkey)
}

func (t *Town) rightsLocked(pubkey string) Rights {
	r := Rights{TownOwner: pubkey == t.data.Owner}
	for _, rm := range t.data.Rooms {
		for _, h := range rm.Houses {
			if h.Owner == pubkey {
				r.Holds = append(r.Holds, h.ID)
			} else if slices.Contains(h.Invited, pubkey) {
				r.Invited = append(r.Invited, h.ID)
			}
		}
	}
	r.Label = r.label()
	return r
}

// View is what a viewer receives: the world as it may see it, plus the
// rights of every visible actor (for labels like [家主] / [オーナー]).
type View struct {
	world.Snapshot
	Rights       map[string]Rights `json:"rights"`
	Applications []Application     `json:"applications,omitempty"` // only for the town owner
}

// ViewFor builds the viewer's view ("" = anonymous).
func (t *Town) ViewFor(viewer string) View {
	s := t.World.ViewSnapshot(viewer)
	t.mu.Lock()
	defer t.mu.Unlock()
	v := View{Snapshot: s, Rights: map[string]Rights{}}
	for _, a := range s.Actors {
		if r := t.rightsLocked(a.ID); r.TownOwner || len(r.Holds) > 0 || len(r.Invited) > 0 {
			v.Rights[a.ID] = r
		}
	}
	if viewer != "" && viewer == t.data.Owner {
		v.Applications = append([]Application{}, t.data.Applications...)
		sort.Slice(v.Applications, func(i, j int) bool { return v.Applications[i].House < v.Applications[j].House })
	}
	return v
}
