package extgate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
)

// Wire constants checked by core on hello (crates/extgate/src/listen/hello.rs).
const (
	Protocol          = 3
	OperationProtocol = 1
)

// Config is what a crab-town instance needs to say hello to core. All values
// come from the operator's admin-side provisioning (instance PUT); none of them
// is a secret.
type Config struct {
	Socket       string `json:"socket"`        // core's gate.listen_socket (absolute path)
	InstanceID   string `json:"instance_id"`   // canonical lowercase UUID
	Revision     uint64 `json:"revision"`      // positive; must equal core's current revision
	ConfigDigest string `json:"config_digest"` // lowerhex SHA-256 of the instance config bytes
	AuthorID     string `json:"author_id"`     // author_id put on every said
	Address      string `json:"address"`       // optional: binding address said goes to (default: first bound)
	Actor        string `json:"actor"`         // optional: actor moved by activity (default: nostarou)
}

// Env keys. CRAB_EXTGATE_CONFIG names a JSON file with the same fields;
// individual env vars override the file.
const (
	EnvConfigFile   = "CRAB_EXTGATE_CONFIG"
	EnvSocket       = "CRAB_EXTGATE_SOCKET"
	EnvInstanceID   = "CRAB_EXTGATE_INSTANCE_ID"
	EnvRevision     = "CRAB_EXTGATE_REVISION"
	EnvConfigDigest = "CRAB_EXTGATE_CONFIG_DIGEST"
	EnvAuthorID     = "CRAB_EXTGATE_AUTHOR_ID"
	EnvAddress      = "CRAB_EXTGATE_ADDRESS"
	EnvActor        = "CRAB_EXTGATE_ACTOR"
)

// LoadConfig reads the extgate config. It returns (nil, nil) when nothing is
// configured: extgate stays disabled and crab-town runs as before. A partial
// or invalid config is an error (fail loud rather than silently disabled).
func LoadConfig(getenv func(string) string) (*Config, error) {
	var c Config
	any := false
	if path := getenv(EnvConfigFile); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("extgate config: %w", err)
		}
		if err := json.Unmarshal(b, &c); err != nil {
			return nil, fmt.Errorf("extgate config: %w", err)
		}
		any = true
	}
	set := func(dst *string, key string) {
		if v := getenv(key); v != "" {
			*dst, any = v, true
		}
	}
	set(&c.Socket, EnvSocket)
	set(&c.InstanceID, EnvInstanceID)
	set(&c.ConfigDigest, EnvConfigDigest)
	set(&c.AuthorID, EnvAuthorID)
	set(&c.Address, EnvAddress)
	set(&c.Actor, EnvActor)
	if v := getenv(EnvRevision); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("extgate config: %s: %w", EnvRevision, err)
		}
		c.Revision, any = n, true
	}
	if !any {
		return nil, nil
	}
	if c.Actor == "" {
		c.Actor = "nostarou"
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Validate checks the shapes core will check on hello.
func (c *Config) Validate() error {
	var errs []error
	if c.Socket == "" {
		errs = append(errs, errors.New("socket is required"))
	}
	if !validUUID(c.InstanceID) {
		errs = append(errs, errors.New("instance_id must be a canonical lowercase UUID"))
	}
	if c.Revision == 0 {
		errs = append(errs, errors.New("revision must be a positive integer"))
	}
	if !validDigest(c.ConfigDigest) {
		errs = append(errs, errors.New("config_digest must be 64 lowercase hex chars"))
	}
	if c.AuthorID == "" {
		errs = append(errs, errors.New("author_id is required"))
	}
	if len(errs) > 0 {
		return fmt.Errorf("extgate config: %w", errors.Join(errs...))
	}
	return nil
}
