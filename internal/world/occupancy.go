package world

import "slices"

// EventOccupancy is emitted when a private zone (toilet, bath) becomes occupied
// or empty. It carries only the in-use door flags -- never who is inside.
const EventOccupancy = "occupancy"

// add records a in the private zone it stands in, if any.
func (o occupancy) add(r *Room, a *Actor) {
	z := r.ZoneAt(a.Pos)
	if z == nil || !z.Private || slices.Contains(o[z.ID], a.ID) {
		return
	}
	o[z.ID] = append(o[z.ID], a.ID)
}

// occupancyLocked: who is inside each private zone of r. Caller holds w.mu.
func (w *World) occupancyLocked(r *Room) occupancy {
	o := occupancy{}
	for _, a := range w.actors {
		if a.RoomID == r.ID {
			o.add(r, a)
		}
	}
	return o
}

// inUseLocked: in-use private zones per room. Caller holds w.mu.
func (w *World) inUseLocked() map[string][]string {
	m := map[string][]string{}
	for id, r := range w.rooms {
		m[id] = w.occupancyLocked(r).inUse()
	}
	return m
}

// occupancyEventsLocked returns one occupancy event per room whose in-use set
// changed since before. Caller holds w.mu.
func (w *World) occupancyEventsLocked(before map[string][]string) []Event {
	var evs []Event
	for id, now := range w.inUseLocked() {
		if !slices.Equal(before[id], now) {
			evs = append(evs, Event{Type: EventOccupancy, Room: id, InUse: now})
		}
	}
	return evs
}
