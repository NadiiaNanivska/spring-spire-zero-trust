package cache

import (
	"os"
	"runtime"
	"testing"
	"time"
)

// freshCache returns a new empty HashCache for isolated unit tests.
// Each test gets its own instance — no global state to reset.
func freshCache() *HashCache {
	return NewHashCache()
}

func writeJar(t *testing.T, content string) string {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "jar-*.jar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func TestHashCache_Miss(t *testing.T) {
	c := freshCache()
	path := writeJar(t, "content-v1")

	hash, err := c.GetOrComputeByPath(path)
	if err != nil {
		t.Fatalf("GetOrComputeByPath: %v", err)
	}
	if hash == "" {
		t.Error("expected non-empty hash")
	}
	if c.Len() != 1 {
		t.Errorf("expected 1 cache entry, got %d", c.Len())
	}
}

func TestHashCache_HitOnUnchangedFile(t *testing.T) {
	c := freshCache()
	path := writeJar(t, "content-v1")

	hash1, err := c.GetOrComputeByPath(path)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	hash2, err := c.GetOrComputeByPath(path)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("unchanged file produced different hashes: %s vs %s", hash1, hash2)
	}
	if c.Len() != 1 {
		t.Errorf("unchanged file should reuse one entry, got %d", c.Len())
	}
}

// TestHashCache_DetectsInPlaceRewriteWithRestoredMtime is the tampering case the
// (inode, mtime) key used to miss: an attacker with write access overwrites the
// jar in place — which preserves the inode — and rewinds mtime with utimensat to
// forge an unchanged key. ctime cannot be forged the same way, so the entry must
// be invalidated and the new content hashed.
func TestHashCache_DetectsInPlaceRewriteWithRestoredMtime(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ctime is only available through syscall.Stat_t on Linux")
	}

	c := freshCache()
	path := writeJar(t, "content-v1")

	hash1, err := c.GetOrComputeByPath(path)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	origMtime := info.ModTime()

	// Same length keeps size identical too, so mtime and size both look untouched.
	if err := os.WriteFile(path, []byte("content-v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, origMtime, origMtime); err != nil {
		t.Fatal(err)
	}

	hash2, err := c.GetOrComputeByPath(path)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if hash1 == hash2 {
		t.Error("stale cache hit after in-place rewrite with restored mtime — ctime is not part of the key")
	}
}

func TestHashCache_InvalidatedOnMtimeChange(t *testing.T) {
	c := freshCache()
	path := writeJar(t, "content-v1")

	hash1, err := c.GetOrComputeByPath(path)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(path, []byte("content-v2-DIFFERENT"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}

	hash2, err := c.GetOrComputeByPath(path)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if hash1 == hash2 {
		t.Error("cache not invalidated after mtime change — expected different hashes")
	}
}

func TestHashCache_MissingFile(t *testing.T) {
	c := freshCache()

	if _, err := c.GetOrComputeByPath("/nonexistent/path/file.jar"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestHashCache_ConcurrentAccess(t *testing.T) {
	// Smoke test: concurrent reads/writes must not race.
	// Run with: go test -race ./internal/...
	c := freshCache()
	path := writeJar(t, "concurrent-content")

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			c.GetOrComputeByPath(path) //nolint:errcheck
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	if c.Len() != 1 {
		t.Errorf("concurrent computation should collapse to one entry, got %d", c.Len())
	}
}
