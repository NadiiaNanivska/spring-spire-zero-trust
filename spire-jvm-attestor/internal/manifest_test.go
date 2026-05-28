package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newManifestCache returns a fresh ManifestCache for isolated unit tests.
// Each test gets its own instance — no shared state to reset.
func newTestManifestCache() *ManifestCache {
	return NewManifestCache()
}

func TestLoadHashManifest_Valid(t *testing.T) {
	cache := newTestManifestCache()

	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	writeManifest(t, path, map[string]string{
		"/app/service.jar": "aabbccdd",
	})

	jars, err := cache.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if jars["/app/service.jar"] != "aabbccdd" {
		t.Errorf("wrong hash: %v", jars)
	}
}

func TestLoadHashManifest_EmptyPath(t *testing.T) {
	cache := newTestManifestCache()
	_, err := cache.Load("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestLoadHashManifest_MissingFile(t *testing.T) {
	cache := newTestManifestCache()
	_, err := cache.Load("/nonexistent/path/manifest.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadHashManifest_InvalidJSON(t *testing.T) {
	cache := newTestManifestCache()

	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, []byte("not json at all {{"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := cache.Load(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadHashManifest_WrongVersion(t *testing.T) {
	cache := newTestManifestCache()

	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	raw, _ := json.Marshal(hashManifest{
		Version: 2, // unsupported
		Jars:    map[string]string{"/app/x.jar": "aabb"},
	})
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := cache.Load(path)
	if err == nil {
		t.Fatal("expected error for unsupported manifest version")
	}
}

func TestLoadHashManifest_EmptyJars(t *testing.T) {
	cache := newTestManifestCache()

	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	raw, _ := json.Marshal(hashManifest{Version: 1, Jars: map[string]string{}})
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := cache.Load(path)
	if err == nil {
		t.Fatal("expected error for manifest with no jar entries")
	}
}

func TestLoadHashManifest_CacheHit(t *testing.T) {
	cache := newTestManifestCache()

	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	writeManifest(t, path, map[string]string{"/app/a.jar": "hash1"})

	// First load — reads from disk.
	jars1, err := cache.Load(path)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}

	// Overwrite the file with different content but restore mtime so the cache
	// key is unchanged — simulates "no change" from the cache's perspective.
	info, _ := os.Stat(path)
	origMtime := info.ModTime()

	writeManifest(t, path, map[string]string{"/app/a.jar": "hash2"})
	if err := os.Chtimes(path, origMtime, origMtime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	// Second load — must return cached (hash1), not the new content (hash2).
	jars2, err := cache.Load(path)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if jars1["/app/a.jar"] != jars2["/app/a.jar"] {
		t.Errorf("cache miss when mtime unchanged: got %v, want %v", jars2, jars1)
	}
}

func TestLoadHashManifest_CacheInvalidatedOnMtimeChange(t *testing.T) {
	cache := newTestManifestCache()

	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	writeManifest(t, path, map[string]string{"/app/a.jar": "hash1"})

	_, err := cache.Load(path)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}

	// Wait so the OS mtime resolution (1 s on many filesystems) advances.
	time.Sleep(10 * time.Millisecond)

	// Overwrite — mtime will be updated by the OS.
	writeManifest(t, path, map[string]string{"/app/a.jar": "hash2"})

	// Advance mtime explicitly to avoid filesystem resolution races.
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	jars2, err := cache.Load(path)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if jars2["/app/a.jar"] != "hash2" {
		t.Errorf("cache not invalidated after mtime change: got %v", jars2)
	}
}