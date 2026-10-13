package config

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/hashicorp/hcl"
)

type Config struct {
	BlockOnAttachSocket bool `hcl:"block_on_attach_socket"`
	// PublicKeys maps key fingerprints (SHA256:<hex>) to base64-encoded Ed25519 public keys.
	PublicKeys map[string]string `hcl:"public_keys"`
}

func Parse(hclData string) (*Config, error) {
	cfg := &Config{}
	if err := hcl.Decode(cfg, hclData); err != nil { return nil, fmt.Errorf("failed to parse plugin config: %w", err) }
	for fingerprint, encoded := range cfg.PublicKeys {
		if !strings.HasPrefix(fingerprint, "SHA256:") || len(strings.TrimPrefix(fingerprint, "SHA256:")) != 64 {
			return nil, fmt.Errorf("public_keys key %q must use SHA256:<64 hex characters>", fingerprint)
		}
		raw, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil || len(raw) != 32 { return nil, fmt.Errorf("public_keys[%q] must be a base64-encoded Ed25519 public key (32 bytes)", fingerprint) }
	}
	return cfg, nil
}
