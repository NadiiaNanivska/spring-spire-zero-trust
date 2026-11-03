package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_ChecksDefaultToEnabled(t *testing.T) {
	cfg, err := Parse(`block_on_attach_socket = false`)
	require.NoError(t, err)
	assert.True(t, cfg.CheckJDK)
	assert.True(t, cfg.CheckLaunch)
	assert.True(t, cfg.CheckJarHash)
	assert.True(t, cfg.CheckProvenance)
}

func TestParse_ChecksCanBeDisabled(t *testing.T) {
	cfg, err := Parse(`
		check_jdk        = true
		check_launch     = true
		check_jar_hash   = false
		check_provenance = false`)
	require.NoError(t, err)
	assert.True(t, cfg.CheckJDK)
	assert.True(t, cfg.CheckLaunch)
	assert.False(t, cfg.CheckJarHash)
	assert.False(t, cfg.CheckProvenance)
}

func TestParse_ProvenanceRequiresJarHash(t *testing.T) {
	_, err := Parse(`check_jar_hash = false`)
	require.Error(t, err)
}
