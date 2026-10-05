package world

// Visitors: actors that are not residents of a house but people who log in
// (the town owner arriving over Nostr with their own avatar).

// Visitor roles (Actor.Role). Residents have Role "".
const (
	RoleOwner = "owner"
	RoleGuest = "guest"
)

// Join adds a visitor actor unless one with the same id is already in the
// world. actsAs, when set, is the id whose house rights the visitor walks
// with: it may enter exactly the houses actsAs may enter (owner / invited);
// the garden is open to everyone. It never lets the visitor command actsAs
// itself. Returns true when the actor was added (an actor event is emitted).
func (w *World) Join(a Actor, actsAs string) (bool, error) {
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
	if a.ID == "" || !r.inBounds(a.Pos) || r.blocked(a.Pos) {
		w.mu.Unlock()
		return false, ErrBadRequest
	}
	if a.State == "" {
		a.State = StateIdle
	}
	if a.Role == "" {
		a.Role = RoleGuest
	}
	a.actsAs, a.pending, a.by, a.path, a.Target, a.Using = actsAs, "", "", nil, nil, ""
	w.actors[a.ID] = &a
	ev := Event{Type: "actor", Room: r.ID, Actor: copyActor(&a), By: a.ID}
	w.mu.Unlock()
	w.emit(ev)
	return true, nil
}

// enterAs is the id whose house rights by walks with (caller holds w.mu).
func (w *World) enterAs(by string) string {
	if a := w.actors[by]; a != nil && a.actsAs != "" {
		return a.actsAs
	}
	return by
}

// PlaceAt names the house and zone of tile p in room ("" when outside any).
func (w *World) PlaceAt(room string, p Pos) (house, zone string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	r := w.rooms[room]
	if r == nil {
		return "", ""
	}
	if h := r.HouseAt(p); h != nil {
		house = h.ID
	}
	if z := r.ZoneAt(p); z != nil {
		zone = z.Name
	}
	return house, zone
}
