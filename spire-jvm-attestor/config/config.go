package config

import (
	"fmt"

	"github.com/hashicorp/hcl"
)

// Config holds plugin configuration from the SPIRE agent.conf plugin_data block.
type Config struct {
	HashManifestPath string `hcl:"hash_manifest_path"`

	// true — fail attestation on Attach API socket; false — emit selector and continue.
	BlockOnAttachSocket bool `hcl:"block_on_attach_socket"`

	// Explicit backend selection: "artifactory" or "manifest".
	// If empty, auto-detected from ArtifactoryURL+ArtifactoryAPIKey presence.
	HashSourceType string `hcl:"hash_source_type"`

	ArtifactoryURL    string `hcl:"artifactory_url"`
	ArtifactoryAPIKey string `hcl:"artifactory_api_key"`
}

func Parse(hclData string) (*Config, error) {
	cfg := &Config{}
	if err := hcl.Decode(cfg, hclData); err != nil {
		return nil, fmt.Errorf("failed to parse plugin config: %w", err)
	}
	return cfg, nil
}
