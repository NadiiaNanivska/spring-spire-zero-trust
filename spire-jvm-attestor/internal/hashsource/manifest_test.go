package hashsource

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalManifestSource_GetExpectedHash(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "manifest-source-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	manifestPath := filepath.Join(tmpDir, "jvm-hashes.json")

	// Схема, що ідеально відповідає generate-manifest.sh: версія є інтом!
	validJSON := `{
		"version": 1,
		"generated_at": "2026-05-30T12:00:00Z",
		"jars": {
			"/app/payments-service.jar": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			"/app/lib-core.jar": "8f434346648f6b96df89d97fa23247ae41e4649b934ca495991b7852b855aaaa"
		}
	}`

	require.NoError(t, os.WriteFile(manifestPath, []byte(validJSON), 0644))

	t.Run("Success: Load and retrieve hash from local manifest", func(t *testing.T) {
		source := NewLocalManifestSource(manifestPath)
		ctx := context.Background()

		hash1, err := source.GetExpectedHash(ctx, "/app/payments-service.jar")
		assert.NoError(t, err)
		assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", hash1)

		hash2, err := source.GetExpectedHash(ctx, "/app/lib-core.jar")
		assert.NoError(t, err)
		assert.Equal(t, "8f434346648f6b96df89d97fa23247ae41e4649b934ca495991b7852b855aaaa", hash2)
	})

	t.Run("Error: JAR file not present in manifest", func(t *testing.T) {
		source := NewLocalManifestSource(manifestPath)
		ctx := context.Background()

		hash, err := source.GetExpectedHash(ctx, "/app/unknown-malicious.jar")
		assert.Error(t, err)
		assert.Empty(t, hash)
		assert.Contains(t, err.Error(), "not found in local manifest")
	})
}
