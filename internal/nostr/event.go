// Package nostr lets crab-town be driven over Nostr: it subscribes to command
// events (ephemeral kind KindCommand) on public relays, verifies their
// signatures, decides who sent them (owner / guest), applies them to the world,
// and publishes the world state as ephemeral KindState events signed with a
// dedicated crab-town key.
package nostr

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// Ephemeral kinds (NIP-01: 20000 <= kind < 30000 is relayed, never stored).
// Picked away from every kind listed in the NIP registry (21059, 22242,
// 23194/23195, 24133, 24242, 27235, 28934-28936).
const (
	KindCommand = 23410 // client -> crab-town: move / snapshot / knock
	KindState   = 23411 // crab-town -> clients: snapshot / world events / results
)

// Event is a NIP-01 event.
type Event struct {
	ID        string     `json:"id"`
	PubKey    string     `json:"pubkey"`
	CreatedAt int64      `json:"created_at"`
	Kind      int        `json:"kind"`
	Tags      [][]string `json:"tags"`
	Content   string     `json:"content"`
	Sig       string     `json:"sig"`
}

var (
	ErrBadID  = errors.New("event id does not match its content")
	ErrBadSig = errors.New("invalid signature")
	ErrFormat = errors.New("malformed event")
)

// serialize is the NIP-01 commitment [0,pubkey,created_at,kind,tags,content]
// encoded like JSON.stringify (no HTML escaping, no spaces).
func (e *Event) serialize() []byte {
	var b strings.Builder
	b.WriteString(`[0,`)
	writeString(&b, e.PubKey)
	b.WriteByte(',')
	b.WriteString(strconv.FormatInt(e.CreatedAt, 10))
	b.WriteByte(',')
	b.WriteString(strconv.Itoa(e.Kind))
	b.WriteString(`,[`)
	for i, tag := range e.Tags {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('[')
		for j, v := range tag {
			if j > 0 {
				b.WriteByte(',')
			}
			writeString(&b, v)
		}
		b.WriteByte(']')
	}
	b.WriteString(`],`)
	writeString(&b, e.Content)
	b.WriteByte(']')
	return []byte(b.String())
}

// writeString writes s as a JSON string the way JSON.stringify does
// (NIP-01 escaping rules: only quote, backslash and control characters).
func writeString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if r < 0x20 {
				fmt.Fprintf(b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

// ComputeID returns the hex sha256 of the NIP-01 serialization.
func (e *Event) ComputeID() string {
	h := sha256.Sum256(e.serialize())
	return hex.EncodeToString(h[:])
}

func isHex(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// Verify checks the shape, that ID commits to the content, and the BIP-340
// signature of PubKey over ID. Any failure means the event must be ignored.
func (e *Event) Verify() error {
	if !isHex(e.ID, 64) || !isHex(e.PubKey, 64) || !isHex(e.Sig, 128) {
		return ErrFormat
	}
	if e.ComputeID() != e.ID {
		return ErrBadID
	}
	pkb, _ := hex.DecodeString(e.PubKey)
	pk, err := schnorr.ParsePubKey(pkb)
	if err != nil {
		return ErrBadSig
	}
	sigb, _ := hex.DecodeString(e.Sig)
	sig, err := schnorr.ParseSignature(sigb)
	if err != nil {
		return ErrBadSig
	}
	idb, _ := hex.DecodeString(e.ID)
	if !sig.Verify(idb, pk) {
		return ErrBadSig
	}
	return nil
}

// Sign fills PubKey, ID and Sig with key.
func (e *Event) Sign(key *btcec.PrivateKey) error {
	e.PubKey = PubHex(key)
	if e.Tags == nil {
		e.Tags = [][]string{}
	}
	e.ID = e.ComputeID()
	idb, _ := hex.DecodeString(e.ID)
	sig, err := schnorr.Sign(key, idb)
	if err != nil {
		return err
	}
	e.Sig = hex.EncodeToString(sig.Serialize())
	return nil
}

// PubHex is the x-only public key of key in hex (the Nostr pubkey).
func PubHex(key *btcec.PrivateKey) string {
	return hex.EncodeToString(schnorr.SerializePubKey(key.PubKey()))
}

// Tag returns the first value of the first tag named name ("" if none).
func (e *Event) Tag(name string) string {
	for _, t := range e.Tags {
		if len(t) >= 2 && t[0] == name {
			return t[1]
		}
	}
	return ""
}
