package config

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/hashicorp/hcl"
)

type Config struct {
	BlockOnAttachSocket bool `hcl:"block_on_attach_socket"`

	// Pipeline stage toggles. Absent keys keep their defaults (see defaultConfig),
	// so existing agent configs keep running the full pipeline.
	CheckJDK        bool `hcl:"check_jdk"`
	CheckLaunch     bool `hcl:"check_launch"`
	CheckJarHash    bool `hcl:"check_jar_hash"`
	CheckProvenance bool `hcl:"check_provenance"`

	// PublicKeys maps key fingerprints (SHA256:<hex>) to base64-encoded Ed25519 public keys.
	PublicKeys map[string]string `hcl:"public_keys"`
}

// defaultConfig enables every check so an omitted key never silently weakens attestation.
func defaultConfig() *Config {
	return &Config{
		CheckJDK:        true,
		CheckLaunch:     true,
		CheckJarHash:    true,
		CheckProvenance: true,
	}
}

func Parse(hclData string) (*Config, error) {
	cfg := defaultConfig()
	if err := hcl.Decode(cfg, hclData); err != nil { return nil, fmt.Errorf("failed to parse plugin config: %w", err) }
	for fingerprint, encoded := range cfg.PublicKeys {
		if !strings.HasPrefix(fingerprint, "SHA256:") || len(strings.TrimPrefix(fingerprint, "SHA256:")) != 64 {
			return nil, fmt.Errorf("public_keys key %q must use SHA256:<64 hex characters>", fingerprint)
		}
		raw, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil || len(raw) != 32 { return nil, fmt.Errorf("public_keys[%q] must be a base64-encoded Ed25519 public key (32 bytes)", fingerprint) }
	}
	// Provenance verifies signatures over hashes produced by the jar-hash stage;
	// without it the stage would silently emit nothing, so reject the combination.
	if cfg.CheckProvenance && !cfg.CheckJarHash {
		return nil, fmt.Errorf("check_provenance requires check_jar_hash = true")
	}
	return cfg, nil
}
