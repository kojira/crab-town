// Package world holds the in-memory state of crab-town: rooms, furniture and actors.
// It knows nothing about HTTP; the server package wraps it.
package world

import (
	"errors"
	"sort"
	"sync"
	"time"
)

const (
	RoomWidth  = 16
	RoomHeight = 12
)

// Actor states.
const (
	StateIdle    = "idle"
	StateWorking = "working"
	StateTalking = "talking"
	StateAway    = "away"
	StateHidden  = "hidden" // only in views: the viewer may not see this actor
)

// Furniture kinds.
const (
	KindWindow      = "window"    // timeline
	KindPC          = "pc"        // work container
	KindBed         = "bed"       // standby
	KindSofa        = "sofa"      // visitors
	KindBookshelf   = "bookshelf" // memory
	KindShoebox     = "shoebox"   // decoration from here on
	KindCounter     = "counter"
	KindStove       = "stove"
	KindFridge      = "fridge"
	KindTable       = "table"
	KindChair       = "chair"
	KindLowTable    = "lowtable"
	KindPlant       = "plant"
	KindNightstand  = "nightstand"
	KindWardrobe    = "wardrobe"
	KindTV          = "tv"
	KindFigureShelf = "figureshelf"
	KindBeanbag     = "beanbag"
	KindGuestBed    = "guestbed"
	KindDoormat     = "doormat"
	KindUmbrella    = "umbrella"
	KindCoatRack    = "coatrack"
	KindKitchenMat  = "kitchenmat"
	KindTrash       = "trash"
	KindClock       = "clock" // wall-mounted
	KindSideboard   = "sideboard"
	KindFloorLamp   = "floorlamp"
	KindRug         = "rug"
	KindDeskChair   = "deskchair"
	KindToilet      = "toilet"
	KindPaper       = "paper"
	KindToiletMat   = "toiletmat"
	KindVanity      = "vanity"
	KindWasher      = "washer"
	KindLaundry     = "laundry"
	KindShower      = "shower"
	KindBathtub     = "bathtub"
	KindBathMat     = "bathmat"
)

var (
	ErrNoActor     = errors.New("actor not found")
	ErrNoRoom      = errors.New("room not found")
	ErrNoFurniture = errors.New("furniture not found")
	ErrOutOfBounds = errors.New("position out of bounds")
	ErrBlocked     = errors.New("position is blocked")
	ErrUnreachable = errors.New("position is unreachable")
	ErrForbidden   = errors.New("forbidden")
	ErrBadRequest  = errors.New("bad request")
	ErrNotUsable   = errors.New("furniture is decoration only")
	ErrNoHouse     = errors.New("house not found")
)

type Pos struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Furniture struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Label    string `json:"label"`
	Function string `json:"function"`           // empty = decoration (cannot be used)
	Pos      Pos    `json:"pos"`                // top-left occupied tile (blocked for walking)
	Size     Size   `json:"size"`               // occupied tiles from Pos (zero = 1x1)
	Access   Pos    `json:"access"`             // tile the actor stands on to use it
	Seat     *Pos   `json:"seat,omitempty"`     // sofa: tile the actor sits on once it arrives (on the furniture)
	State    string `json:"state"`              // actor state while using it
	Walkable bool   `json:"walkable,omitempty"` // flat (rug, mat, desk chair): does not block
}

// Occupies reports whether the furniture covers tile p.
func (f *Furniture) Occupies(p Pos) bool {
	w, h := f.Size.W, f.Size.H
	if w <= 0 {
		w = 1
	}
	if h <= 0 {
		h = 1
	}
	return Rect{f.Pos.X, f.Pos.Y, w, h}.Contains(p)
}

// Room is a walkable map (the town). Ownership lives on its Houses.
type Room struct {
	ID         string       `json:"id"`
	Width      int          `json:"width"`
	Height     int          `json:"height"`
	Visibility string       `json:"visibility"` // public / owner / invited
	Houses     []*House     `json:"houses"`
	Zones      []*Zone      `json:"zones"`
	Walls      []Rect       `json:"walls"` // impassable
	Doors      []Pos        `json:"doors"` // passable gaps in walls
	Furniture  []*Furniture `json:"furniture"`

	HiddenZones []string `json:"hidden_zones,omitempty"` // view only: zones the viewer may not see
	InUse       []string `json:"in_use,omitempty"`       // view only: occupied private zones (door lamp)
}

type Actor struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	RoomID string `json:"room"`
	Pos    Pos    `json:"pos"`
	State  string `json:"state"`
	Using  string `json:"using,omitempty"`  // furniture id currently in use
	Target *Pos   `json:"target,omitempty"` // walking destination
	Hidden bool   `json:"hidden,omitempty"` // view only: position/state withheld from this viewer
	Pubkey string `json:"pubkey,omitempty"` // visitor's Nostr pubkey (hex): the viewer shows its kind:0 picture
	Role   string `json:"role,omitempty"`   // "" = resident; visitors: RoleOwner | RoleGuest (Join)

	actsAs  string // visitor: whose house rights it walks with (Join)
	pending string // furniture to use on arrival
	by      string // who requested the current walk
	path    []Pos
}

// Event is something that happened in the world. Sent to WS subscribers and the webhook hook.
type Event struct {
	Type      string     `json:"type"` // actor | interact | knock | talk | say | occupancy
	Time      time.Time  `json:"time"`
	Room      string     `json:"room,omitempty"`
	Actor     *Actor     `json:"actor,omitempty"`
	Furniture *Furniture `json:"furniture,omitempty"`
	By        string     `json:"by,omitempty"`
	Message   string     `json:"message,omitempty"`
	House     string     `json:"house,omitempty"`    // knock: the house knocked on
	To        string     `json:"to,omitempty"`       // talk: the actor spoken to
	Role      string     `json:"role,omitempty"`     // talk: owner | guest (as the caller was verified)
	ReplyTo   string     `json:"reply_to,omitempty"` // say: the talker (by) the words answer
	Image     string     `json:"image,omitempty"`    // talk: an https image URL (owner only)
	// occupancy events only: occupied private zones, and (per viewer) hidden zones
	InUse       []string `json:"in_use,omitempty"`
	HiddenZones []string `json:"hidden_zones,omitempty"`
}

// Snapshot is the full world state.
type Snapshot struct {
	Type   string   `json:"type"` // "snapshot"
	Viewer string   `json:"viewer,omitempty"`
	Rooms  []*Room  `json:"rooms"`
	Actors []*Actor `json:"actors"`
}

type World struct {
	mu     sync.Mutex
	rooms  map[string]*Room
	actors map[string]*Actor
	subs   map[chan Event]struct{}
	hook   func(Event)
	now    func() time.Time
}

func New() *World {
	return &World{
		rooms:  map[string]*Room{},
		actors: map[string]*Actor{},
		subs:   map[chan Event]struct{}{},
		now:    time.Now,
	}
}

// SetHook registers a callback invoked (outside the world lock) for every event.
func (w *World) SetHook(fn func(Event)) {
	w.mu.Lock()
	w.hook = fn
	w.mu.Unlock()
}

func (w *World) AddRoom(r *Room) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if r.Width == 0 {
		r.Width = RoomWidth
	}
	if r.Height == 0 {
		r.Height = RoomHeight
	}
	w.rooms[r.ID] = r
}

func (w *World) AddActor(a *Actor) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if a.State == "" {
		a.State = StateIdle
	}
	w.actors[a.ID] = a
}

// Subscribe returns a channel receiving events and a cancel func.
func (w *World) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 64)
	w.mu.Lock()
	w.subs[ch] = struct{}{}
	w.mu.Unlock()
	return ch, func() {
		w.mu.Lock()
		if _, ok := w.subs[ch]; ok {
			delete(w.subs, ch)
			close(ch)
		}
		w.mu.Unlock()
	}
}

// Snapshot returns a copy of the world state (rooms and actors sorted by id).
func (w *World) Snapshot() Snapshot {
	w.mu.Lock()
	defer w.mu.Unlock()
	s := Snapshot{Type: "snapshot", Rooms: []*Room{}, Actors: []*Actor{}}
	for _, r := range w.rooms {
		rc := *r
		rc.Houses = nil
		for _, h := range r.Houses {
			hc := *h
			hc.Invited = append([]string{}, h.Invited...)
			rc.Houses = append(rc.Houses, &hc)
		}
		rc.Zones = nil
		for _, z := range r.Zones {
			zc := *z
			rc.Zones = append(rc.Zones, &zc)
		}
		rc.Walls = append([]Rect{}, r.Walls...)
		rc.Doors = append([]Pos{}, r.Doors...)
		rc.Furniture = nil
		for _, f := range r.Furniture {
			fc := *f
			rc.Furniture = append(rc.Furniture, &fc)
		}
		s.Rooms = append(s.Rooms, &rc)
	}
	for _, a := range w.actors {
		s.Actors = append(s.Actors, copyActor(a))
	}
	sort.Slice(s.Rooms, func(i, j int) bool { return s.Rooms[i].ID < s.Rooms[j].ID })
	sort.Slice(s.Actors, func(i, j int) bool { return s.Actors[i].ID < s.Actors[j].ID })
	return s
}

// Actor returns a copy of the actor.
func (w *World) Actor(id string) (Actor, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	a, ok := w.actors[id]
	if !ok {
		return Actor{}, false
	}
	return *copyActor(a), true
}

func copyActor(a *Actor) *Actor {
	c := *a
	if a.Target != nil {
		t := *a.Target
		c.Target = &t
	}
	c.path = nil
	return &c
}

func (r *Room) inBounds(p Pos) bool {
	return p.X >= 0 && p.Y >= 0 && p.X < r.Width && p.Y < r.Height
}

func (r *Room) blocked(p Pos) bool {
	if r.IsWall(p) {
		return true
	}
	for _, f := range r.Furniture {
		if !f.Walkable && f.Occupies(p) {
			return true
		}
	}
	return false
}

func (r *Room) furniture(id string) *Furniture {
	for _, f := range r.Furniture {
		if f.ID == id {
			return f
		}
	}
	return nil
}

// findPath does BFS on the room grid (4-neighbour). Returned path excludes start.
func (r *Room) findPath(from, to Pos) ([]Pos, bool) {
	if from == to {
		return nil, true
	}
	prev := map[Pos]Pos{from: from}
	q := []Pos{from}
	dirs := []Pos{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	for len(q) > 0 {
		cur := q[0]
		q = q[1:]
		for _, d := range dirs {
			n := Pos{cur.X + d.X, cur.Y + d.Y}
			if !r.inBounds(n) || r.blocked(n) {
				continue
			}
			if _, seen := prev[n]; seen {
				continue
			}
			prev[n] = cur
			if n == to {
				var path []Pos
				for p := n; p != from; p = prev[p] {
					path = append([]Pos{p}, path...)
				}
				return path, true
			}
			q = append(q, n)
		}
	}
	return nil, false
}

func (w *World) lookup(actorID string) (*Actor, *Room, error) {
	a, ok := w.actors[actorID]
	if !ok {
		return nil, nil, ErrNoActor
	}
	r, ok := w.rooms[a.RoomID]
	if !ok {
		return nil, nil, ErrNoRoom
	}
	return a, r, nil
}

// Move sets a walking destination for the actor. by = requester id.
func (w *World) Move(by, actorID string, to Pos) error {
	w.mu.Lock()
	a, r, err := w.lookup(actorID)
	if err != nil {
		w.mu.Unlock()
		return err
	}
	if !r.canCommand(by, a) || (r.inBounds(to) && !r.canEnter(w.enterAs(by), to)) {
		w.mu.Unlock()
		return ErrForbidden
	}
	if !r.inBounds(to) {
		w.mu.Unlock()
		return ErrOutOfBounds
	}
	if r.blocked(to) {
		w.mu.Unlock()
		return ErrBlocked
	}
	path, ok := r.findPath(a.Pos, to)
	if !ok {
		w.mu.Unlock()
		return ErrUnreachable
	}
	a.Using = ""
	a.pending = ""
	a.by = by
	a.State = StateIdle
	a.path = path
	a.Target = nil
	if len(path) > 0 {
		t := to
		a.Target = &t
	}
	ev := Event{Type: "actor", Room: r.ID, Actor: copyActor(a), By: by}
	w.mu.Unlock()
	w.emit(ev)
	return nil
}

// Interact makes the actor walk to the furniture and use it on arrival.
func (w *World) Interact(by, actorID, furnitureID string) error {
	w.mu.Lock()
	a, r, err := w.lookup(actorID)
	if err != nil {
		w.mu.Unlock()
		return err
	}
	if !r.canCommand(by, a) {
		w.mu.Unlock()
		return ErrForbidden
	}
	f := r.furniture(furnitureID)
	if f == nil {
		w.mu.Unlock()
		return ErrNoFurniture
	}
	if f.Function == "" {
		w.mu.Unlock()
		return ErrNotUsable
	}
	if !r.canEnter(w.enterAs(by), f.Access) {
		w.mu.Unlock()
		return ErrForbidden
	}
	path, ok := r.findPath(a.Pos, f.Access)
	if !ok {
		w.mu.Unlock()
		return ErrUnreachable
	}
	a.Using = ""
	a.pending = f.ID
	a.by = by
	a.path = path
	var evs []Event
	if len(path) == 0 {
		evs = w.arriveLocked(a, r)
	} else {
		t := f.Access
		a.Target = &t
		a.State = StateIdle
		evs = []Event{{Type: "actor", Room: r.ID, Actor: copyActor(a), By: by}}
	}
	w.mu.Unlock()
	for _, ev := range evs {
		w.emit(ev)
	}
	return nil
}

// Knock: anyone with an id may knock on a house (by house id) or a room.
// Emits a knock event for the owner.
func (w *World) Knock(by, target, message string) error {
	if by == "" {
		return ErrBadRequest
	}
	w.mu.Lock()
	ev := Event{Type: "knock", By: by, Message: message}
	if r, ok := w.rooms[target]; ok {
		ev.Room = r.ID
	} else {
		for _, r := range w.rooms {
			if h := r.House(target); h != nil {
				ev.Room, ev.House = r.ID, h.ID
			}
		}
	}
	if ev.Room == "" {
		w.mu.Unlock()
		return ErrNoRoom
	}
	w.mu.Unlock()
	w.emit(ev)
	return nil
}

// arriveLocked starts using the pending furniture. Caller holds w.mu.
func (w *World) arriveLocked(a *Actor, r *Room) []Event {
	a.Target = nil
	a.path = nil
	f := r.furniture(a.pending)
	a.pending = ""
	if f == nil {
		a.State = StateIdle
		return []Event{{Type: "actor", Room: r.ID, Actor: copyActor(a), By: a.by}}
	}
	a.Using = f.ID
	a.State = f.State
	if f.Seat != nil { // sit on the furniture itself, not in front of it
		a.Pos = *f.Seat
	}
	fc := *f
	return []Event{
		{Type: "actor", Room: r.ID, Actor: copyActor(a), By: a.by},
		{Type: "interact", Room: r.ID, Actor: copyActor(a), Furniture: &fc, By: a.by},
	}
}

// Step advances every walking actor by one tile.
func (w *World) Step() {
	w.mu.Lock()
	before := w.inUseLocked()
	var evs []Event
	for _, a := range w.actors {
		if len(a.path) == 0 {
			continue
		}
		r := w.rooms[a.RoomID]
		a.Pos = a.path[0]
		a.path = a.path[1:]
		if len(a.path) == 0 {
			if a.pending != "" && r != nil {
				evs = append(evs, w.arriveLocked(a, r)...)
				continue
			}
			a.Target = nil
		}
		evs = append(evs, Event{Type: "actor", Room: a.RoomID, Actor: copyActor(a), By: a.by})
	}
	evs = append(evs, w.occupancyEventsLocked(before)...)
	w.mu.Unlock()
	for _, ev := range evs {
		w.emit(ev)
	}
}

func (w *World) emit(ev Event) {
	ev.Time = w.now()
	w.mu.Lock()
	for ch := range w.subs {
		select {
		case ch <- ev:
		default: // slow subscriber: drop (it can resync via snapshot)
		}
	}
	hook := w.hook
	w.mu.Unlock()
	if hook != nil {
		hook(ev)
	}
}
