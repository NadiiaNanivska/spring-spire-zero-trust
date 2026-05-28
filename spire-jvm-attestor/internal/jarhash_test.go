package internal

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// ── helpers ──────────────────────────────────────────────────────────────────

// fakeJarContent is a small deterministic byte sequence used as a fake jar.
var fakeJarContent = []byte("PK\x03\x04fake-jar-content-for-testing")

// sha256hex returns the SHA-256 hex digest of b.
func sha256hex(b []byte) string {
	h := sha256.New()
	h.Write(b)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// writeManifest writes a JSON manifest to path with the given jars map.
func writeManifest(t *testing.T, path string, jars map[string]string) {
	t.Helper()
	m := hashManifest{
		Version:     1,
		GeneratedBy: "test",
		Jars:        jars,
	}
	raw, _ := json.Marshal(m)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir manifest dir: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

// runJarHash is a test helper that calls JarHashChecker.Check with fresh,
// isolated cache instances. Mirrors the old verifyJarHash(procRoot, manifestPath)
// signature.
func runJarHash(t *testing.T, procRoot, manifestPath string) ([]string, error) {
	t.Helper()
	checker := NewJarHashChecker(manifestPath)
	ctx := &AttestationContext{
		ProcRoot:      procRoot,
		HashCache:     NewHashCache(),
		ManifestCache: NewManifestCache(),
	}
	return checker.Check(ctx)
}

// makeProcRootWithMaps creates a fake procRoot directory with:
//   - maps file referencing jarContainerPath with the real on-disk inode
//   - root/<jarContainerPath> containing the actual jar bytes
//
// Returns procRoot and the inode of the jar file (from syscall.Stat_t).
func makeProcRootWithMaps(t *testing.T, jarContainerPath string, jarContent []byte) (procRoot string, inode uint64) {
	t.Helper()
	dir := t.TempDir()

	// Write the jar inside procRoot/root/<jarContainerPath>.
	jarFSPath := filepath.Join(dir, "root", jarContainerPath)
	if err := os.MkdirAll(filepath.Dir(jarFSPath), 0o755); err != nil {
		t.Fatalf("mkdir jar dir: %v", err)
	}
	if err := os.WriteFile(jarFSPath, jarContent, 0o644); err != nil {
		t.Fatalf("write jar: %v", err)
	}

	// Get the real inode of the jar.
	info, err := os.Stat(jarFSPath)
	if err != nil {
		t.Fatalf("stat jar: %v", err)
	}
	sys := info.Sys().(*syscall.Stat_t)
	inode = sys.Ino

	// Write /proc/<PID>/maps referencing the jar with its correct inode.
	mapsContent := fmt.Sprintf(
		"7f0000000000-7f0010000000 r--p 00000000 fd:01 %d %s\n",
		inode,
		jarContainerPath,
	)
	if err := os.WriteFile(filepath.Join(dir, "maps"), []byte(mapsContent), 0o644); err != nil {
		t.Fatalf("write maps: %v", err)
	}

	return dir, inode
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestVerifyJarHash_HappyPath(t *testing.T) {
	const jarPath = "/app/service.jar"

	procRoot, _ := makeProcRootWithMaps(t, jarPath, fakeJarContent)
	hash := sha256hex(fakeJarContent)

	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	writeManifest(t, manifestPath, map[string]string{jarPath: hash})

	selectors, err := runJarHash(t, procRoot, manifestPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsSelector(selectors, "jvm:maps_verified=true") {
		t.Errorf("expected maps_verified=true, got %v", selectors)
	}
	if !containsSelector(selectors, "jvm:inode_consistent=true") {
		t.Errorf("expected inode_consistent=true, got %v", selectors)
	}
	if !containsSelector(selectors, "jvm:jar_sha256:"+hash) {
		t.Errorf("expected jvm:jar_sha256:%s in selectors, got %v", hash, selectors)
	}
}

func TestVerifyJarHash_HashMismatch(t *testing.T) {
	const jarPath = "/app/service.jar"

	procRoot, _ := makeProcRootWithMaps(t, jarPath, fakeJarContent)

	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	writeManifest(t, manifestPath, map[string]string{jarPath: "deadbeefdeadbeef"})

	_, err := runJarHash(t, procRoot, manifestPath)
	if err == nil {
		t.Fatal("expected error on hash mismatch, got nil")
	}
}

func TestVerifyJarHash_JarNotInManifest(t *testing.T) {
	const jarPath = "/app/service.jar"

	procRoot, _ := makeProcRootWithMaps(t, jarPath, fakeJarContent)

	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	writeManifest(t, manifestPath, map[string]string{"/app/other.jar": "aabbcc"})

	_, err := runJarHash(t, procRoot, manifestPath)
	if err == nil {
		t.Fatal("expected error when jar not in manifest")
	}
}

func TestVerifyJarHash_InodeConsistentFalse_BaitAndSwitch(t *testing.T) {
	// Simulate bait-and-switch: maps records inode=999999999 (the "good" jar)
	// but disk now has a different file at the same path (inode doesn't match).
	const jarPath = "/app/service.jar"

	dir := t.TempDir()

	jarFSPath := filepath.Join(dir, "root", jarPath)
	if err := os.MkdirAll(filepath.Dir(jarFSPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(jarFSPath, fakeJarContent, 0o644); err != nil {
		t.Fatalf("write jar: %v", err)
	}

	// Write maps with a deliberately wrong inode (simulates bait-and-switch).
	fakeInode := uint64(999999999)
	mapsContent := fmt.Sprintf(
		"7f0000000000-7f0010000000 r--p 00000000 fd:01 %d %s\n",
		fakeInode,
		jarPath,
	)
	if err := os.WriteFile(filepath.Join(dir, "maps"), []byte(mapsContent), 0o644); err != nil {
		t.Fatalf("write maps: %v", err)
	}

	hash := sha256hex(fakeJarContent)
	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	writeManifest(t, manifestPath, map[string]string{jarPath: hash})

	selectors, err := runJarHash(t, dir, manifestPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return inode_consistent=false — policy decides what to do.
	if !containsSelector(selectors, "jvm:inode_consistent=false") {
		t.Errorf("expected inode_consistent=false for bait-and-switch, got %v", selectors)
	}
}

func TestVerifyJarHash_SpringBootFallback(t *testing.T) {
	// Spring Boot: jar not in maps, but found via -jar in cmdline (inode=0).
	const jarPath = "/app/application.jar"

	dir := t.TempDir()

	// Empty maps — no jar entries.
	if err := os.WriteFile(filepath.Join(dir, "maps"), []byte(""), 0o644); err != nil {
		t.Fatalf("write maps: %v", err)
	}

	// Write cmdline with -jar.
	cmdline := fmt.Sprintf("java\x00-jar\x00%s\x00", jarPath)
	if err := os.WriteFile(filepath.Join(dir, "cmdline"), []byte(cmdline), 0o644); err != nil {
		t.Fatalf("write cmdline: %v", err)
	}

	// Place the jar at procRoot/root/<jarPath>.
	jarFSPath := filepath.Join(dir, "root", jarPath)
	if err := os.MkdirAll(filepath.Dir(jarFSPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(jarFSPath, fakeJarContent, 0o644); err != nil {
		t.Fatalf("write jar: %v", err)
	}

	hash := sha256hex(fakeJarContent)
	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	writeManifest(t, manifestPath, map[string]string{jarPath: hash})

	selectors, err := runJarHash(t, dir, manifestPath)
	if err != nil {
		t.Fatalf("Spring Boot fallback failed: %v", err)
	}

	if !containsSelector(selectors, "jvm:maps_verified=true") {
		t.Errorf("expected maps_verified=true, got %v", selectors)
	}
	// inode=0 path skips the TOCTOU check → should still report consistent.
	if !containsSelector(selectors, "jvm:inode_consistent=true") {
		t.Errorf("expected inode_consistent=true for Spring Boot path, got %v", selectors)
	}
}

func TestVerifyJarHash_NoJarsAnywhere(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "maps"), []byte(""), 0o644); err != nil {
		t.Fatalf("write maps: %v", err)
	}
	cmdline := "java\x00-cp\x00/app/classes\x00com.example.Main\x00"
	if err := os.WriteFile(filepath.Join(dir, "cmdline"), []byte(cmdline), 0o644); err != nil {
		t.Fatalf("write cmdline: %v", err)
	}

	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	writeManifest(t, manifestPath, map[string]string{"/app/service.jar": "aabbcc"})

	_, err := runJarHash(t, dir, manifestPath)
	if err == nil {
		t.Fatal("expected error when no jars found anywhere")
	}
}

func TestVerifyJarHash_CacheHit(t *testing.T) {
	// Verify that a second call with the same jar uses the hash cache.
	// We call the same JarHashChecker (with the same HashCache) twice and confirm
	// both calls return consistent results.
	const jarPath = "/app/cached.jar"

	procRoot, _ := makeProcRootWithMaps(t, jarPath, fakeJarContent)
	hash := sha256hex(fakeJarContent)

	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	writeManifest(t, manifestPath, map[string]string{jarPath: hash})

	// Use a single checker+cache pair for both calls to exercise the cache hit.
	checker := NewJarHashChecker(manifestPath)
	sharedCtx := &AttestationContext{
		ProcRoot:      procRoot,
		HashCache:     NewHashCache(),
		ManifestCache: NewManifestCache(),
	}

	// First call — populates the hash cache.
	selectors, err := checker.Check(sharedCtx)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if !containsSelector(selectors, "jvm:jar_sha256:"+hash) {
		t.Fatalf("first call: wrong hash selector, got %v", selectors)
	}

	// Second call — should hit the cache (no re-read).
	selectors2, err := checker.Check(sharedCtx)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	if !containsSelector(selectors2, "jvm:jar_sha256:"+hash) {
		t.Errorf("second call: expected same hash, got %v", selectors2)
	}
}

func TestVerifyJarHash_MissingManifest(t *testing.T) {
	const jarPath = "/app/service.jar"
	procRoot, _ := makeProcRootWithMaps(t, jarPath, fakeJarContent)

	_, err := runJarHash(t, procRoot, "/nonexistent/manifest.json")
	if err == nil {
		t.Fatal("expected error for missing manifest")
	}
}

// TestSha256File ensures the low-level hasher produces correct output.
func TestSha256File(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "jar-*.jar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(f, "hello"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	got, err := sha256File(f.Name())
	if err != nil {
		t.Fatalf("sha256File: %v", err)
	}

	// Known SHA-256 of "hello".
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got != want {
		t.Errorf("sha256File = %s, want %s", got, want)
	}
}