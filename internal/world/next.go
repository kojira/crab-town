package world

import "errors"

// Hooks for the next-generation town (internal/next). They are additive:
// nothing in the current town (cmd/crab-town, internal/server, internal/nostr,
// internal/extgate) calls them, so its behaviour does not change.

// EventLeave: an actor left the world (next-generation town only).
const EventLeave = "leave"

// SetHolder replaces the holder (House.Owner) of a house, looked up by house
// id only. "" makes it a vacant plot: nobody may enter or see its private zones.
func (w *World) SetHolder(house, holder string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, r := range w.rooms {
		if h := r.House(house); h != nil {
			h.Owner = holder
			return nil
		}
	}
	return ErrNoHouse
}

// Appear adds an actor unless one with the same id already exists. Unlike
// Join it sets no Role and no borrowed house rights: the actor is just an id
// whose rights are the houses it holds or is invited to. Pubkey must be set,
// so the actor can only ever be moved by itself (canCommand). If a.Pos is
// taken by another actor it appears on the nearest free tile reachable from
// it without entering a house it may not (decided under the world lock, so
// simultaneous joins never stack). No free tile = ErrTileTaken.
// Returns true when added (an actor event is emitted).
func (w *World) Appear(a Actor) (bool, error) {
	if a.ID == "" || a.Pubkey == "" {
		return false, ErrBadRequest
	}
	w.mu.Lock()
	if _, ok := w.actors[a.ID]; ok {
		w.mu.Unlock()
		return false, nil
	}
	r, ok := w.rooms[a.RoomID]
	if !ok {
		w.mu.Unlock()
		return false, ErrNoRoom
	}
	if !r.inBounds(a.Pos) || r.blocked(a.Pos) {
		w.mu.Unlock()
		return false, ErrBadRequest
	}
	free, ok := w.freeTileLocked(r, a.Pos, a.ID)
	if !ok {
		w.mu.Unlock()
		return false, ErrTileTaken
	}
	a.Pos = free
	if a.State == "" {
		a.State = StateIdle
	}
	a.Role, a.actsAs, a.pending, a.by, a.path, a.Target, a.Using = "", "", "", "", nil, nil, ""
	w.actors[a.ID] = &a
	ev := Event{Type: "actor", Room: r.ID, Actor: copyActor(&a), By: a.ID}
	w.mu.Unlock()
	w.emit(ev)
	return true, nil
}

// Remove takes an actor out of the world (emits EventLeave).
func (w *World) Remove(id string) error {
	w.mu.Lock()
	a, ok := w.actors[id]
	if !ok {
		w.mu.Unlock()
		return ErrNoActor
	}
	before := w.inUseLocked()
	delete(w.actors, id)
	evs := []Event{{Type: EventLeave, Room: a.RoomID, Actor: copyActor(a), By: id}}
	evs = append(evs, w.occupancyEventsLocked(before)...)
	w.mu.Unlock()
	for _, ev := range evs {
		w.emit(ev)
	}
	return nil
}

// ActorCount is the number of actors in the world.
func (w *World) ActorCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.actors)
}

// ErrInUse: the furniture is being used (or walked to) by another actor.
var ErrInUse = errors.New("furniture is in use by someone else")

// InteractExclusive is Interact for the next-generation town, plus one rule:
// a usable piece of furniture serves one actor at a time. If another actor is
// already using it or walking to it, the call fails with ErrInUse. The check
// and the claim happen under one lock, so of two simultaneous callers exactly
// one wins. (Interact itself is unchanged: the current town keeps sharing.)
func (w *World) InteractExclusive(by, actorID, furnitureID string) error {
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
	for _, o := range w.actors {
		if o != a && o.RoomID == r.ID && (o.Using == f.ID || o.pending == f.ID) {
			w.mu.Unlock()
			return ErrInUse
		}
	}
	path, ok := r.findPath(a.Pos, f.Access)
	if !ok {
		w.mu.Unlock()
		return ErrUnreachable
	}
	a.Using, a.pending, a.by, a.path = "", f.ID, by, path
	var evs []Event
	if len(path) == 0 {
		evs = w.arriveLocked(a, r)
	} else {
		t := f.Access
		a.Target, a.State = &t, StateIdle
		evs = []Event{{Type: "actor", Room: r.ID, Actor: copyActor(a), By: by}}
	}
	w.mu.Unlock()
	for _, ev := range evs {
		w.emit(ev)
	}
	return nil
}

// ErrTileTaken: another actor stands on (or is walking to) the destination.
var ErrTileTaken = errors.New("tile is taken by another actor")

// MoveExclusive is Move for the next-generation town, plus one rule: two
// actors may not end up on the same tile. A destination where another actor
// stands still, or is walking to, fails with ErrTileTaken. Passing through
// each other on the way is allowed (no deadlocks in corridors). Check and
// claim happen under one lock. (Move itself is unchanged.)
func (w *World) MoveExclusive(by, actorID string, to Pos) error {
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
	if w.takenLocked(r, to, a) {
		w.mu.Unlock()
		return ErrTileTaken
	}
	path, ok := r.findPath(a.Pos, to)
	if !ok {
		w.mu.Unlock()
		return ErrUnreachable
	}
	a.Using, a.pending, a.by, a.State, a.path, a.Target = "", "", by, StateIdle, path, nil
	if len(path) > 0 {
		t := to
		a.Target = &t
	}
	ev := Event{Type: "actor", Room: r.ID, Actor: copyActor(a), By: by}
	w.mu.Unlock()
	w.emit(ev)
	return nil
}

// takenLocked: another actor than self stands still on p or is walking to it.
func (w *World) takenLocked(r *Room, p Pos, self *Actor) bool {
	for _, o := range w.actors {
		if o == self || o.RoomID != r.ID {
			continue
		}
		if (o.Target != nil && *o.Target == p) || (len(o.path) == 0 && o.Pos == p) {
			return true
		}
	}
	return false
}

// freeTileLocked: from, or the nearest walkable tile reachable from it that
// nobody has taken (BFS). It never spills into a house id may not enter.
// Caller holds w.mu.
func (w *World) freeTileLocked(r *Room, from Pos, id string) (Pos, bool) {
	seen := map[Pos]bool{from: true}
	q := []Pos{from}
	for len(q) > 0 {
		cur := q[0]
		q = q[1:]
		if !w.takenLocked(r, cur, nil) {
			return cur, true
		}
		for _, d := range []Pos{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			n := Pos{cur.X + d.X, cur.Y + d.Y}
			if r.inBounds(n) && !r.blocked(n) && !seen[n] && r.canEnter(id, n) {
				seen[n] = true
				q = append(q, n)
			}
		}
	}
	return Pos{}, false
}
