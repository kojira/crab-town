package world

import "slices"

// House is one home inside a room (the town). Zones belong to a house via
// Zone.House; a zone with no house (the garden) is outdoors. Visibility of
// invited / owner zones and the right to operate (move actors, use furniture)
// are decided per house: its owner and its invited ids.
type House struct {
	ID      string   `json:"id"`
	Owner   string   `json:"owner"`
	Invited []string `json:"invited"`
	Rect    Rect     `json:"rect"` // footprint including the outer walls
}

// CanOperate: the owner or an invited id (nil house = nobody).
func (h *House) CanOperate(by string) bool {
	if by == "" || h == nil {
		return false
	}
	return by == h.Owner || slices.Contains(h.Invited, by)
}

// House returns the house with the given id, or nil.
func (r *Room) House(id string) *House {
	for _, h := range r.Houses {
		if h.ID == id {
			return h
		}
	}
	return nil
}

// houseOf resolves a house id or an owner id to the house.
func (r *Room) houseOf(key string) *House {
	if h := r.House(key); h != nil {
		return h
	}
	for _, h := range r.Houses {
		if h.Owner == key {
			return h
		}
	}
	return nil
}

// HouseAt returns the house whose footprint contains p, or nil (garden).
func (r *Room) HouseAt(p Pos) *House {
	for _, h := range r.Houses {
		if h.Rect.Contains(p) {
			return h
		}
	}
	return nil
}

// zoneHouse returns the house a zone belongs to (nil = outdoors).
func (r *Room) zoneHouse(z *Zone) *House {
	if z.House == "" {
		return nil
	}
	return r.House(z.House)
}

// canEnter: by may send an actor onto p. Outdoors (garden) any identified
// caller may; inside a house only its owner and invited ids.
func (r *Room) canEnter(by string, p Pos) bool {
	if by == "" {
		return false
	}
	if h := r.HouseAt(p); h != nil {
		return h.CanOperate(by)
	}
	return true
}

// canCommand: by may give orders to actor a -- a itself, or whoever may
// operate the house a currently stands in (the host or an invited id).
func (r *Room) canCommand(by string, a *Actor) bool {
	if by == "" {
		return false
	}
	return by == a.ID || r.HouseAt(a.Pos).CanOperate(by)
}

// SetInvited replaces the invited ids of a house (by house id or owner id).
func (w *World) SetInvited(house string, ids []string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, r := range w.rooms {
		if h := r.houseOf(house); h != nil {
			h.Invited = append([]string{}, ids...)
			return nil
		}
	}
	return ErrNoHouse
}

// House returns the house with the given id (any room), or nil. For tests and
// setup only: the returned pointer is live world state.
func (w *World) House(id string) *House {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, r := range w.rooms {
		if h := r.House(id); h != nil {
			return h
		}
	}
	return nil
}

// shiftZones moves a house layout right by dx tiles and tags it with house.
func shiftZones(zs []*Zone, dx int, house string) []*Zone {
	for _, z := range zs {
		z.Rect.X += dx
		z.House = house
	}
	return zs
}

func shiftRects(rs []Rect, dx int) []Rect {
	for i := range rs {
		rs[i].X += dx
	}
	return rs
}

func shiftPos(ps []Pos, dx int) []Pos {
	for i := range ps {
		ps[i].X += dx
	}
	return ps
}

func shiftFurniture(fs []*Furniture, dx int) []*Furniture {
	for _, f := range fs {
		f.Pos.X += dx
		if f.Function != "" {
			f.Access.X += dx
		}
	}
	return fs
}
