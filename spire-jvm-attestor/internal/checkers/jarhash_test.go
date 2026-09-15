package checkers

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourorg/spire-jvm-attestor/internal/cache"
)

func TestJarHashChecker_Check(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "jarhash-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	procRoot := filepath.Join(tmpDir, "proc", "1234")
	nsRoot := filepath.Join(procRoot, "root")
	appDir := filepath.Join(nsRoot, "app")
	require.NoError(t, os.MkdirAll(appDir, 0755))

	jar1Path := filepath.Join(appDir, "service.jar")
	jar2Path := filepath.Join(appDir, "lib-core.jar")
	jar1NsPath := "/app/service.jar"
	jar2NsPath := "/app/lib-core.jar"

	require.NoError(t, os.WriteFile(jar1Path, []byte("fake-jar-1-content"), 0644))
	require.NoError(t, os.WriteFile(jar2Path, []byte("fake-jar-2-content"), 0644))

	hash1 := computeRawSHA256([]byte("fake-jar-1-content"))
	hash2 := computeRawSHA256([]byte("fake-jar-2-content"))

	stat1, err := os.Stat(jar1Path)
	require.NoError(t, err)
	ino1 := getInode(stat1)

	stat2, err := os.Stat(jar2Path)
	require.NoError(t, err)
	ino2 := getInode(stat2)

	checker := NewJarHashChecker()
	hashCache := cache.NewHashCache()

	t.Run("Success: Multi-JAR verification (Fix Bug 8)", func(t *testing.T) {
		ctx := &AttestationContext{
			Context:   context.Background(),
			PID:       1234,
			ProcRoot:  procRoot,
			HashCache: hashCache,
		}

		createFakeMapsFile(t, procRoot, ino1, jar1NsPath, ino2, jar2NsPath)

		selectors, err := checker.Check(ctx)
		assert.NoError(t, err)

		assert.Contains(t, selectors, SelectorJarSha256Prefix+hash1)
		assert.Contains(t, selectors, SelectorJarSha256Prefix+hash2)
		assert.Contains(t, selectors, SelectorMapsVerified)
		assert.Contains(t, selectors, SelectorInodeConsistentTrue)
	})

	t.Run("Success: Spring Boot Fat-JAR support (Fix Bug 9)", func(t *testing.T) {
		ctx := &AttestationContext{
			Context:   context.Background(),
			PID:       1234,
			ProcRoot:  procRoot,
			HashCache: cache.NewHashCache(),
		}

		createFakeMapsFile(t, procRoot, 0, jar1NsPath, 0, "")

		selectors, err := checker.Check(ctx)
		assert.NoError(t, err)
		assert.Contains(t, selectors, SelectorJarSha256Prefix+hash1)
		assert.Contains(t, selectors, SelectorInodeConsistentTrue)
	})

	t.Run("Success: OverlayFS Copy-Up Adaptation (Fix Issue 4)", func(t *testing.T) {
		ctx := &AttestationContext{
			Context:   context.Background(),
			PID:       1234,
			ProcRoot:  procRoot,
			HashCache: hashCache,
		}

		createFakeMapsFile(t, procRoot, 99999, jar1NsPath, 0, "")

		selectors, err := checker.Check(ctx)
		assert.NoError(t, err)
		assert.Contains(t, selectors, SelectorJarSha256Prefix+hash1)
		assert.Contains(t, selectors, SelectorInodeConsistentFalse)
	})

	t.Run("Modified JAR: emits recomputed hash, no hard-fail", func(t *testing.T) {
		ctx := &AttestationContext{
			Context:   context.Background(),
			PID:       1234,
			ProcRoot:  procRoot,
			HashCache: cache.NewHashCache(),
		}

		modifiedContent := []byte("MALICIOUS_BYTECODE_INJECTED")
		require.NoError(t, os.WriteFile(jar1Path, modifiedContent, 0644))
		statMod, err := os.Stat(jar1Path)
		require.NoError(t, err)

		createFakeMapsFile(t, procRoot, getInode(statMod), jar1NsPath, 0, "")

		modifiedHash := computeRawSHA256(modifiedContent)
		selectors, err := checker.Check(ctx)
		assert.NoError(t, err)
		assert.Contains(t, selectors, SelectorJarSha256Prefix+modifiedHash)
		assert.NotContains(t, selectors, SelectorJarSha256Prefix+hash1)
	})
}
