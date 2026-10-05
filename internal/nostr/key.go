package nostr

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// LoadOrCreateKey reads crab-town's own signing key (hex) from path, creating
// a fresh random key with mode 0600 if the file does not exist. A key file
// readable by group/others is refused. The secret is never logged or returned
// in any printable form; callers only ever see the public key.
func LoadOrCreateKey(path string) (*btcec.PrivateKey, bool, error) {
	if path == "" {
		return nil, false, errors.New("nostr: key file path is empty")
	}
	st, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		key, err := btcec.NewPrivateKey()
		if err != nil {
			return nil, false, err
		}
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return nil, false, fmt.Errorf("nostr: create key file: %w", err)
		}
		_, werr := f.WriteString(hex.EncodeToString(key.Serialize()) + "\n")
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			os.Remove(path)
			return nil, false, fmt.Errorf("nostr: write key file: %w", werr)
		}
		return key, true, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("nostr: key file: %w", err)
	}
	if st.Mode().Perm()&0o077 != 0 {
		return nil, false, fmt.Errorf("nostr: key file %s must be mode 0600 (is %o)", path, st.Mode().Perm())
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("nostr: key file: %w", err)
	}
	raw, err := hex.DecodeString(strings.TrimSpace(string(b)))
	if err != nil || len(raw) != 32 {
		return nil, false, errors.New("nostr: key file must hold 32 bytes of hex")
	}
	key, _ := btcec.PrivKeyFromBytes(raw)
	return key, false, nil
}

// ParsePubKey accepts a 64-char hex pubkey or an npub and returns lowercase hex.
func ParsePubKey(s string) (string, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if strings.HasPrefix(s, "npub1") {
		b, err := decodeNpub(s)
		if err != nil {
			return "", err
		}
		s = hex.EncodeToString(b)
	}
	if !isHex(s, 64) {
		return "", fmt.Errorf("nostr: invalid pubkey %q", s)
	}
	raw, _ := hex.DecodeString(s)
	if _, err := schnorr.ParsePubKey(raw); err != nil {
		return "", fmt.Errorf("nostr: pubkey not on curve: %w", err)
	}
	return s, nil
}

const bech32Charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

func bech32Polymod(values []byte) uint32 {
	gen := [5]uint32{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
	chk := uint32(1)
	for _, v := range values {
		top := chk >> 25
		chk = (chk&0x1ffffff)<<5 ^ uint32(v)
		for i := 0; i < 5; i++ {
			if (top>>uint(i))&1 == 1 {
				chk ^= gen[i]
			}
		}
	}
	return chk
}

// decodeNpub decodes a bech32 npub (NIP-19) to its 32-byte payload,
// verifying the checksum.
func decodeNpub(s string) ([]byte, error) {
	pos := strings.LastIndexByte(s, '1')
	if pos < 1 || pos+7 > len(s) {
		return nil, errors.New("nostr: bad npub")
	}
	hrp, data := s[:pos], s[pos+1:]
	if hrp != "npub" {
		return nil, errors.New("nostr: not an npub")
	}
	vals := make([]byte, len(data))
	for i := range data {
		j := strings.IndexByte(bech32Charset, data[i])
		if j < 0 {
			return nil, errors.New("nostr: bad npub character")
		}
		vals[i] = byte(j)
	}
	exp := make([]byte, 0, len(hrp)*2+1+len(vals))
	for i := range hrp {
		exp = append(exp, hrp[i]>>5)
	}
	exp = append(exp, 0)
	for i := range hrp {
		exp = append(exp, hrp[i]&31)
	}
	if bech32Polymod(append(exp, vals...)) != 1 {
		return nil, errors.New("nostr: npub checksum mismatch")
	}
	var out []byte
	acc, bits := 0, 0
	for _, v := range vals[:len(vals)-6] {
		acc = acc<<5 | int(v)
		bits += 5
		for bits >= 8 {
			bits -= 8
			out = append(out, byte(acc>>bits))
		}
	}
	if len(out) != 32 {
		return nil, errors.New("nostr: npub payload must be 32 bytes")
	}
	return out, nil
}
