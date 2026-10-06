// Package next is the next-generation town, run side by side with the current
// one (cmd/crab-town, port 8787, tokens.json), which it never touches. It is a
// separate binary (cmd/town-next, port 8788) and reuses only the world engine
// (walking, walls, zone visibility) and the Nostr signature check.
//
// Differences from the current town:
//   - identity is one Nostr pubkey; every acting request is signed (NIP-98).
//     There are no tokens.
//   - nobody is "AI" or "human", "resident" or "guest": an actor is a pubkey.
//     What it may do depends only on permissions: town owner, house holder,
//     invited. The actor id IS the pubkey (hex).
//   - houses, holders and known names are data (a JSON file), not code. A
//     vacant plot is a house with no holder; anyone may apply, the town owner
//     approves.
package next

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kojira/crab-town/internal/world"
)

// Data is the whole town as stored on disk. It holds no secret.
type Data struct {
	Owner        string        `json:"owner"`                // town owner pubkey (hex): approves plots
	Spawn        Spawn         `json:"spawn"`                // where a newcomer appears
	MaxActors    int           `json:"max_actors,omitempty"` // 0 = DefaultMaxActors
	Names        []Name        `json:"names,omitempty"`      // display names / home spots of known pubkeys
	Applications []Application `json:"applications,omitempty"`
	Rooms        []*world.Room `json:"rooms"` // geometry + houses (holder = House.Owner, "" = vacant)
}

// Spawn is a tile in a room.
type Spawn struct {
	Room string    `json:"room"`
	Pos  world.Pos `json:"pos"`
}

// Name is how the town shows a pubkey, and where it appears on joining (Room
// "" = the spawn). It grants nothing: rights come only from houses / owner.
type Name struct {
	Pubkey string    `json:"pubkey"`
	Name   string    `json:"name"`
	Room   string    `json:"room,omitempty"`
	Pos    world.Pos `json:"pos,omitempty"`
}

// Application: pubkey asked for the vacant plot House (pending approval).
type Application struct {
	House  string `json:"house"`
	Pubkey string `json:"pubkey"`
}

const DefaultMaxActors = 64

func isPubHex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func (d *Data) room(id string) *world.Room {
	for _, r := range d.Rooms {
		if r.ID == id {
			return r
		}
	}
	return nil
}

func (d *Data) house(id string) *world.House {
	for _, r := range d.Rooms {
		if h := r.House(id); h != nil {
			return h
		}
	}
	return nil
}

func inRoom(r *world.Room, p world.Pos) bool {
	return p.X >= 0 && p.Y >= 0 && p.X < r.Width && p.Y < r.Height
}

// Validate rejects a file the town could not run safely with.
func (d *Data) Validate() error {
	if !isPubHex(d.Owner) {
		return errors.New("owner: want a 64-char lowercase hex pubkey")
	}
	if len(d.Rooms) == 0 {
		return errors.New("rooms: none")
	}
	houses := map[string]bool{}
	for _, r := range d.Rooms {
		if r.ID == "" || r.Width <= 0 || r.Height <= 0 {
			return fmt.Errorf("room %q: id and size required", r.ID)
		}
		for _, h := range r.Houses {
			if h.ID == "" || houses[h.ID] {
				return fmt.Errorf("house %q: empty or duplicate id", h.ID)
			}
			houses[h.ID] = true
			if h.Owner != "" && !isPubHex(h.Owner) {
				return fmt.Errorf("house %s: holder must be a hex pubkey or empty (vacant)", h.ID)
			}
			for _, id := range h.Invited {
				if !isPubHex(id) {
					return fmt.Errorf("house %s: invited %q is not a hex pubkey", h.ID, id)
				}
			}
		}
	}
	if r := d.room(d.Spawn.Room); r == nil || !inRoom(r, d.Spawn.Pos) {
		return errors.New("spawn: unknown room or out of bounds")
	}
	seen := map[string]bool{}
	for _, n := range d.Names {
		if !isPubHex(n.Pubkey) || seen[n.Pubkey] {
			return fmt.Errorf("names: bad or duplicate pubkey %q", n.Pubkey)
		}
		seen[n.Pubkey] = true
		if n.Room != "" {
			if r := d.room(n.Room); r == nil || !inRoom(r, n.Pos) {
				return fmt.Errorf("names %s: unknown room or out of bounds", n.Name)
			}
		}
	}
	for _, a := range d.Applications {
		if !houses[a.House] || !isPubHex(a.Pubkey) {
			return fmt.Errorf("applications: bad entry %+v", a)
		}
	}
	return nil
}

// Load reads and validates a town file.
func Load(path string) (*Data, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var d Data
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := d.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &d, nil
}

// Save writes the file atomically (temp file in the same dir + rename).
func (d *Data) Save(path string) error {
	b, err := json.MarshalIndent(d, "", " ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".town-next-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// FromWorld exports the rooms of an existing world as town data (the one-time
// migration of the hard-coded town). holders maps a current owner id (e.g.
// "nostarou") to its pubkey; an owner without a mapping becomes a vacant plot.
// Invited ids are dropped (they were ids, not pubkeys). A mapped actor's name
// and position become its Names entry (where it appears on joining).
func FromWorld(w *world.World, owner string, spawn Spawn, holders map[string]string) *Data {
	d := &Data{Owner: owner, Spawn: spawn}
	s := w.Snapshot()
	for _, a := range s.Actors {
		if pk := holders[a.ID]; pk != "" {
			d.Names = append(d.Names, Name{Pubkey: pk, Name: a.Name, Room: a.RoomID, Pos: a.Pos})
		}
	}
	for _, r := range s.Rooms {
		r.HiddenZones, r.InUse = nil, nil
		for _, h := range r.Houses {
			h.Owner, h.Invited = holders[h.Owner], []string{}
		}
		d.Rooms = append(d.Rooms, r)
	}
	return d
}
