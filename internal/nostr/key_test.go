package nostr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const kojiraNpub = "npub1k0jrarx8um0lyw3nmysn50539ky4k8p7gfgzgrsvn8d7lccx3d0s38dczd"
const kojiraHex = "b3e43e8cc7e6dff23a33d9213a3e912d895b1c3e4250240e0c99dbefe3068b5f"

func TestParsePubKey(t *testing.T) {
	for _, in := range []string{kojiraNpub, kojiraHex, strings.ToUpper(kojiraHex)} {
		got, err := ParsePubKey(in)
		if err != nil || got != kojiraHex {
			t.Fatalf("ParsePubKey(%s) = %s, %v", in, got, err)
		}
	}
	bad := kojiraNpub[:len(kojiraNpub)-1] + "q" // checksum broken
	for _, in := range []string{bad, "npub1xyz", kojiraHex[:62], "zz" + kojiraHex[2:]} {
		if _, err := ParsePubKey(in); err == nil {
			t.Fatalf("ParsePubKey(%s) accepted", in)
		}
	}
}

func TestKeyFileIsCreated0600AndReused(t *testing.T) {
	p := filepath.Join(t.TempDir(), "town.key")
	k1, created, err := LoadOrCreateKey(p)
	if err != nil || !created {
		t.Fatalf("create: %v %v", created, err)
	}
	st, _ := os.Stat(p)
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %o", st.Mode().Perm())
	}
	k2, created, err := LoadOrCreateKey(p)
	if err != nil || created || PubHex(k1) != PubHex(k2) {
		t.Fatalf("reload: %v %v", created, err)
	}
	os.Chmod(p, 0o644)
	if _, _, err := LoadOrCreateKey(p); err == nil {
		t.Fatal("a world-readable key file must be refused")
	}
}

func TestLoadConfig(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	if c, err := LoadConfig(env(nil)); c != nil || err != nil {
		t.Fatalf("disabled: %v %v", c, err)
	}
	c, err := LoadConfig(env(map[string]string{"CRAB_NOSTR_KEY_FILE": "k", "CRAB_NOSTR_OWNER": kojiraNpub}))
	if err != nil || c.Owner != kojiraHex || len(c.Relays) != len(DefaultRelays) || c.Window != 2*time.Minute {
		t.Fatalf("defaults: %+v %v", c, err)
	}
	c, err = LoadConfig(env(map[string]string{"CRAB_NOSTR_KEY_FILE": "k", "CRAB_NOSTR_RELAYS": "wss://a, wss://b", "CRAB_NOSTR_WINDOW": "30s"}))
	if err != nil || len(c.Relays) != 2 || c.Relays[1] != "wss://b" || c.Window != 30*time.Second || c.Owner != "" {
		t.Fatalf("custom: %+v %v", c, err)
	}
	for _, m := range []map[string]string{
		{"CRAB_NOSTR_KEY_FILE": "k", "CRAB_NOSTR_RELAYS": "https://a"},
		{"CRAB_NOSTR_KEY_FILE": "k", "CRAB_NOSTR_OWNER": "npub1bad"},
		{"CRAB_NOSTR_KEY_FILE": "k", "CRAB_NOSTR_WINDOW": "-1s"},
	} {
		if _, err := LoadConfig(env(m)); err == nil {
			t.Fatalf("accepted %v", m)
		}
	}
}

// No owner configured: nobody is the owner, so every pubkey is a guest.
func TestNoOwnerMeansEveryoneIsGuest(t *testing.T) {
	h := &Handler{}
	if h.Role(kojiraHex) != RoleGuest || h.Role("") != RoleGuest {
		t.Fatal("empty owner must not match")
	}
}
