package internal

import (
	"os"
	"testing"
	"time"
)

// freshCache returns a new empty HashCache for isolated unit tests.
// Each test gets its own instance — no global state to reset.
func freshCache() *HashCache {
	return NewHashCache()
}

func TestHashCache_Miss(t *testing.T) {
	c := freshCache()

	f, err := os.CreateTemp(t.TempDir(), "jar-*.jar")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("content-v1")
	f.Close()

	hash, err := c.GetOrCompute(f.Name())
	if err != nil {
		t.Fatalf("GetOrCompute: %v", err)
	}
	if hash == "" {
		t.Error("expected non-empty hash")
	}
}

func TestHashCache_Hit(t *testing.T) {
	c := freshCache()

	f, err := os.CreateTemp(t.TempDir(), "jar-*.jar")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("content-v1")
	f.Close()

	hash1, err := c.GetOrCompute(f.Name())
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	// Overwrite with different content but restore original mtime so the
	// cache key (inode+mtime) stays the same — simulates "no file change".
	info, _ := os.Stat(f.Name())
	origMtime := info.ModTime()

	if err := os.WriteFile(f.Name(), []byte("content-v2-DIFFERENT"), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Chtimes(f.Name(), origMtime, origMtime)

	hash2, err := c.GetOrCompute(f.Name())
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	// Should return cached value (hash1), not the new file content.
	if hash1 != hash2 {
		t.Errorf("cache miss when key unchanged: got %s, want %s", hash2, hash1)
	}
}

func TestHashCache_InvalidatedOnMtimeChange(t *testing.T) {
	c := freshCache()

	f, err := os.CreateTemp(t.TempDir(), "jar-*.jar")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("content-v1")
	f.Close()

	hash1, err := c.GetOrCompute(f.Name())
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	// Change the file AND advance mtime so the cache key changes.
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(f.Name(), []byte("content-v2-DIFFERENT"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	os.Chtimes(f.Name(), future, future)

	hash2, err := c.GetOrCompute(f.Name())
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if hash1 == hash2 {
		t.Error("cache not invalidated after mtime change — expected different hashes")
	}
}

func TestHashCache_MissingFile(t *testing.T) {
	c := freshCache()
	_, err := c.GetOrCompute("/nonexistent/path/file.jar")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestHashCache_ConcurrentAccess(t *testing.T) {
	// Smoke test: concurrent reads/writes must not race.
	// Run with: go test -race ./internal/...
	c := freshCache()

	f, err := os.CreateTemp(t.TempDir(), "jar-*.jar")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("concurrent-content")
	f.Close()

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			c.GetOrCompute(f.Name()) //nolint:errcheck
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}