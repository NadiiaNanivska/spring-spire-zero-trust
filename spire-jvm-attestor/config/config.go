package config

import (
	"fmt"

	"github.com/hashicorp/hcl"
)

type Config struct {
	// False emits the attach-socket selector without rejecting attestation.
	BlockOnAttachSocket bool `hcl:"block_on_attach_socket"`
}

func Parse(hclData string) (*Config, error) {
	cfg := &Config{}
	if err := hcl.Decode(cfg, hclData); err != nil {
		return nil, fmt.Errorf("failed to parse plugin config: %w", err)
	}
	return cfg, nil
}
