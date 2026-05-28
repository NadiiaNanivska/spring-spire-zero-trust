package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func writeMaps(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "maps"), []byte(content), 0o644); err != nil {
		t.Fatalf("write maps: %v", err)
	}
	return dir
}

func TestParseJarPathsFromMaps_SingleJar(t *testing.T) {
	procRoot := writeMaps(t, `7f3a00000000-7f3a10000000 r--p 00000000 fd:01 1234567 /app/service.jar
7f3a10000000-7f3a20000000 r-xp 00000000 fd:01 1234567 /app/service.jar
7fff00000000-7fff10000000 r--p 00000000 00:00 0 [stack]
`)

	entries, err := parseJarPathsFromMaps(procRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d: %+v", len(entries), entries)
	}
	if entries[0].path != "/app/service.jar" {
		t.Errorf("expected /app/service.jar, got %s", entries[0].path)
	}
	if entries[0].inode != 1234567 {
		t.Errorf("expected inode 1234567, got %d", entries[0].inode)
	}
}

func TestParseJarPathsFromMaps_DeduplicatesJar(t *testing.T) {
	// Same jar mapped multiple times (read + exec segments) — should yield 1 entry
	procRoot := writeMaps(t, `7f3a00000000-7f3a10000000 r--p 00000000 fd:01 999 /app/fat.jar
7f3a10000000-7f3a20000000 r-xp 01000000 fd:01 999 /app/fat.jar
7f3a20000000-7f3a30000000 rw-p 02000000 fd:01 999 /app/fat.jar
`)

	entries, err := parseJarPathsFromMaps(procRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 deduplicated entry, got %d", len(entries))
	}
}

func TestParseJarPathsFromMaps_NoJars(t *testing.T) {
	procRoot := writeMaps(t, `7f3a00000000-7f3a10000000 r-xp 00000000 fd:01 111 /lib/libc.so.6
7fff00000000-7fff10000000 r--p 00000000 00:00 0 [stack]
`)

	entries, err := parseJarPathsFromMaps(procRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no entries, got %+v", entries)
	}
}

func TestExtractJarsFromCmdline_SpringBoot(t *testing.T) {
	dir := t.TempDir()
	cmdline := "java\x00-Xmx512m\x00-jar\x00/app/application.jar\x00"
	if err := os.WriteFile(filepath.Join(dir, "cmdline"), []byte(cmdline), 0o644); err != nil {
		t.Fatalf("write cmdline: %v", err)
	}

	entries, err := extractJarsFromCmdline(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].path != "/app/application.jar" {
		t.Errorf("expected /app/application.jar, got %s", entries[0].path)
	}
	if entries[0].inode != 0 {
		t.Errorf("cmdline fallback should set inode=0, got %d", entries[0].inode)
	}
}

func TestExtractJarsFromCmdline_NoJar(t *testing.T) {
	dir := t.TempDir()
	cmdline := "java\x00-cp\x00/app/classes\x00com.example.Main\x00"
	if err := os.WriteFile(filepath.Join(dir, "cmdline"), []byte(cmdline), 0o644); err != nil {
		t.Fatalf("write cmdline: %v", err)
	}

	entries, err := extractJarsFromCmdline(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no entries for classpath launch, got %+v", entries)
	}
}
