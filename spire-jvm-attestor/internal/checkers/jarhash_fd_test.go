package checkers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourorg/spire-jvm-attestor/internal/cache"
	"github.com/yourorg/spire-jvm-attestor/internal/procfs"
)

func fakeProcWithFDs(t *testing.T, jarPaths ...string) string {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("fd discovery relies on POSIX symlink semantics")
	}

	procRoot := filepath.Join(t.TempDir(), "1234")
	fdDir := filepath.Join(procRoot, "fd")
	require.NoError(t, os.MkdirAll(fdDir, 0o755))

	require.NoError(t, os.WriteFile(
		filepath.Join(procRoot, "maps"),
		[]byte("7f3a00000000-7f3a10000000 r-xp 00000000 fd:01 111 /lib/libc.so.6\n"),
		0o644,
	))

	for i, jarPath := range jarPaths {
		require.NoError(t, os.Symlink(jarPath, filepath.Join(fdDir, itoa(i+3))))
	}
	return procRoot
}

func fakeProcWithMapsAndFDs(t *testing.T, mappedJars, openJars []string) string {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("discovery relies on POSIX symlink semantics")
	}

	procRoot := filepath.Join(t.TempDir(), "1234")
	fdDir := filepath.Join(procRoot, "fd")
	mapFilesDir := filepath.Join(procRoot, "map_files")
	require.NoError(t, os.MkdirAll(fdDir, 0o755))
	require.NoError(t, os.MkdirAll(mapFilesDir, 0o755))

	var maps strings.Builder
	maps.WriteString("7f3a00000000-7f3a10000000 r-xp 00000000 fd:01 111 /lib/libc.so.6\n")

	for i, jarPath := range mappedJars {
		fi, err := os.Stat(jarPath)
		require.NoError(t, err)
		inode, err := cache.GetInode(fi)
		require.NoError(t, err)

		start := (i + 1) * 0x400000
		addrRange := fmt.Sprintf("%08x-%08x", start, start+0x52000)
		fmt.Fprintf(&maps, "%s r-xp 00000000 08:02 %d %s\n", addrRange, inode, jarPath)
		require.NoError(t, os.Symlink(jarPath, filepath.Join(mapFilesDir, addrRange)))
	}

	require.NoError(t, os.WriteFile(filepath.Join(procRoot, "maps"), []byte(maps.String()), 0o644))

	for i, jarPath := range openJars {
		require.NoError(t, os.Symlink(jarPath, filepath.Join(fdDir, itoa(i+3))))
	}

	return procRoot
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func writeJarFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func selectorWithPrefix(selectors []string, prefix string) string {
	for _, s := range selectors {
		if strings.HasPrefix(s, prefix) {
			return s
		}
	}
	return ""
}

func TestJarHashChecker_FallsBackToFDTable(t *testing.T) {
	appDir := t.TempDir()
	jarPath := writeJarFile(t, appDir, "payments-service.jar", "fat-jar-bytecode")
	procRoot := fakeProcWithFDs(t, jarPath)

	selectors, err := NewJarHashChecker().Check(&AttestationContext{
		Context:   context.Background(),
		PID:       1234,
		ProcRoot:  procRoot,
		HashCache: cache.NewHashCache(),
	})
	require.NoError(t, err)

	assert.Contains(t, selectors, SelectorJarSha256Prefix+computeRawSHA256([]byte("fat-jar-bytecode")))
	assert.Contains(t, selectors, SelectorJarSourcePrefix+"fd")
	assert.Contains(t, selectors, SelectorMapsVerified)
	assert.Contains(t, selectors, SelectorKernelHandleTrue)
}

// Use different namespace bytes to detect fallback; ordinary symlinks cannot emulate /proc fd binding.
func TestJarHashChecker_ReadsThroughKernelHandleNotPathname(t *testing.T) {
	appDir := t.TempDir()
	jarPath := writeJarFile(t, appDir, "payments-service.jar", "original-bytecode")
	procRoot := fakeProcWithFDs(t, jarPath)

	nsPath := filepath.Join(procRoot, "root", jarPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(nsPath), 0o755))
	require.NoError(t, os.WriteFile(nsPath, []byte("attacker-bytecode"), 0o644))

	selectors, err := NewJarHashChecker().Check(&AttestationContext{
		Context:   context.Background(),
		PID:       1234,
		ProcRoot:  procRoot,
		HashCache: cache.NewHashCache(),
	})
	require.NoError(t, err)

	assert.Contains(t, selectors, SelectorJarSha256Prefix+computeRawSHA256([]byte("original-bytecode")),
		"hash must come from the descriptor the process holds, not from the pathname")
	assert.NotContains(t, selectors, SelectorJarSha256Prefix+computeRawSHA256([]byte("attacker-bytecode")))
	assert.Contains(t, selectors, SelectorKernelHandleTrue)
}

func TestJarHashChecker_DecoyJarChangesSetDigest(t *testing.T) {
	appDir := t.TempDir()
	cleanPath := writeJarFile(t, appDir, "clean.jar", "clean-bytecode")
	evilPath := writeJarFile(t, appDir, "evil.jar", "evil-bytecode")

	check := func(jars ...string) []string {
		selectors, err := NewJarHashChecker().Check(&AttestationContext{
			Context:   context.Background(),
			PID:       1234,
			ProcRoot:  fakeProcWithFDs(t, jars...),
			HashCache: cache.NewHashCache(),
		})
		require.NoError(t, err)
		return selectors
	}

	honest := check(cleanPath)
	tampered := check(cleanPath, evilPath)

	cleanSelector := SelectorJarSha256Prefix + computeRawSHA256([]byte("clean-bytecode"))
	assert.Contains(t, honest, cleanSelector)
	assert.Contains(t, tampered, cleanSelector,
		"the decoy leaves the clean per-jar selector in place — this is the bypass")

	honestSet := selectorWithPrefix(honest, SelectorJarSetSha256Prefix)
	tamperedSet := selectorWithPrefix(tampered, SelectorJarSetSha256Prefix)
	require.NotEmpty(t, honestSet)
	assert.NotEqual(t, honestSet, tamperedSet,
		"the set digest must change when an extra jar appears")
}

func TestJarHashChecker_MapsDoesNotShadowFDTable(t *testing.T) {
	appDir := t.TempDir()
	approved := writeJarFile(t, appDir, "payments-service.jar", "approved-bytecode")
	evil := writeJarFile(t, appDir, "evil.jar", "evil-bytecode")

	procRoot := fakeProcWithMapsAndFDs(t, []string{approved}, []string{evil})

	selectors, err := NewJarHashChecker().Check(&AttestationContext{
		Context:   context.Background(),
		PID:       1234,
		ProcRoot:  procRoot,
		HashCache: cache.NewHashCache(),
	})
	require.NoError(t, err)

	assert.Contains(t, selectors, SelectorJarSha256Prefix+computeRawSHA256([]byte("approved-bytecode")))
	assert.Contains(t, selectors, SelectorJarSha256Prefix+computeRawSHA256([]byte("evil-bytecode")),
		"a jar held open via fd must not be hidden by a mapped one")
	assert.Contains(t, selectors, SelectorJarSourcePrefix+procfs.SourceMapsAndFD)

	clean := sha256.Sum256([]byte(approved + ":" + computeRawSHA256([]byte("approved-bytecode")) + "\n"))
	assert.NotContains(t, selectors, SelectorJarSetSha256Prefix+hex.EncodeToString(clean[:]),
		"the set digest must not still equal the clean single-jar value")
}

func TestJarHashChecker_SameJarMappedAndOpenCountsOnce(t *testing.T) {
	appDir := t.TempDir()
	jarPath := writeJarFile(t, appDir, "payments-service.jar", "fat-jar-bytecode")

	procRoot := fakeProcWithMapsAndFDs(t, []string{jarPath}, []string{jarPath})

	selectors, err := NewJarHashChecker().Check(&AttestationContext{
		Context:   context.Background(),
		PID:       1234,
		ProcRoot:  procRoot,
		HashCache: cache.NewHashCache(),
	})
	require.NoError(t, err)

	var perJar int
	for _, selector := range selectors {
		if strings.HasPrefix(selector, SelectorJarSha256Prefix) {
			perJar++
		}
	}
	assert.Equal(t, 1, perJar, "one jar reached through two sources is still one jar")

	wire := jarPath + ":" + computeRawSHA256([]byte("fat-jar-bytecode")) + "\n"
	sum := sha256.Sum256([]byte(wire))
	assert.Contains(t, selectors, SelectorJarSetSha256Prefix+hex.EncodeToString(sum[:]))
}

// Keep this digest fixture identical to wsldev/apps.TestJarSetDigest_MatchesPluginWireFormat.
func TestJarHashChecker_SetDigestWireFormat(t *testing.T) {
	appDir := t.TempDir()
	jarPath := writeJarFile(t, appDir, "payments-service.jar", "fat-jar-bytecode")
	procRoot := fakeProcWithFDs(t, jarPath)

	selectors, err := NewJarHashChecker().Check(&AttestationContext{
		Context:   context.Background(),
		PID:       1234,
		ProcRoot:  procRoot,
		HashCache: cache.NewHashCache(),
	})
	require.NoError(t, err)

	wire := jarPath + ":" + computeRawSHA256([]byte("fat-jar-bytecode")) + "\n"
	sum := sha256.Sum256([]byte(wire))

	assert.Contains(t, selectors, SelectorJarSetSha256Prefix+hex.EncodeToString(sum[:]))
}

func TestJarHashChecker_CmdlineFallbackIsNotVerified(t *testing.T) {
	procRoot := filepath.Join(t.TempDir(), "1234")
	appDir := filepath.Join(procRoot, "root", "app")
	require.NoError(t, os.MkdirAll(appDir, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(procRoot, "fd"), 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(procRoot, "maps"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(
		filepath.Join(procRoot, "cmdline"),
		[]byte("java\x00-jar\x00/app/service.jar\x00"),
		0o644,
	))
	require.NoError(t, os.WriteFile(filepath.Join(appDir, "service.jar"), []byte("bytes"), 0o644))

	selectors, err := NewJarHashChecker().Check(&AttestationContext{
		Context:   context.Background(),
		PID:       1234,
		ProcRoot:  procRoot,
		HashCache: cache.NewHashCache(),
	})
	require.NoError(t, err)

	assert.Contains(t, selectors, SelectorJarSourcePrefix+"cmdline")
	assert.Contains(t, selectors, SelectorMapsVerifiedFalse)
	assert.Contains(t, selectors, SelectorKernelHandleFalse)
	assert.NotContains(t, selectors, SelectorMapsVerified)
}

func TestJarHashChecker_NotAJVMProcess(t *testing.T) {
	procRoot := filepath.Join(t.TempDir(), "1234")
	require.NoError(t, os.MkdirAll(procRoot, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(procRoot, "maps"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(procRoot, "cmdline"), []byte("nginx\x00-g\x00daemon off;\x00"), 0o644))

	_, err := NewJarHashChecker().Check(&AttestationContext{
		Context:   context.Background(),
		PID:       1234,
		ProcRoot:  procRoot,
		HashCache: cache.NewHashCache(),
	})
	assert.ErrorIs(t, err, ErrNotJVM)
}
