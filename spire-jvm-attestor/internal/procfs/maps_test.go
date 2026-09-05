package procfs

import (
	"os"
	"path/filepath"
	"runtime"
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

	entries, err := ParseJarPathsFromMaps(procRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d: %+v", len(entries), entries)
	}
	if entries[0].Path != "/app/service.jar" {
		t.Errorf("expected /app/service.jar, got %s", entries[0].Path)
	}
	if entries[0].Inode != 1234567 {
		t.Errorf("expected inode 1234567, got %d", entries[0].Inode)
	}
	if entries[0].Source != SourceMaps {
		t.Errorf("expected source %q, got %q", SourceMaps, entries[0].Source)
	}
	// The handle must name the first mapping's address range so the caller can
	// read the exact inode without resolving the pathname.
	wantHandle := filepath.Join(procRoot, "map_files", "7f3a00000000-7f3a10000000")
	if entries[0].KernelPath != wantHandle {
		t.Errorf("expected kernel handle %s, got %s", wantHandle, entries[0].KernelPath)
	}
}

func TestParseJarPathsFromMaps_DeduplicatesJar(t *testing.T) {
	// Same jar mapped multiple times (read + exec segments) — should yield 1 entry
	procRoot := writeMaps(t, `7f3a00000000-7f3a10000000 r--p 00000000 fd:01 999 /app/fat.jar
7f3a10000000-7f3a20000000 r-xp 01000000 fd:01 999 /app/fat.jar
7f3a20000000-7f3a30000000 rw-p 02000000 fd:01 999 /app/fat.jar
`)

	entries, err := ParseJarPathsFromMaps(procRoot)
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

	entries, err := ParseJarPathsFromMaps(procRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no entries, got %+v", entries)
	}
}

// fakeFDTable builds a /proc/<PID>/fd-like directory of symlinks to real files.
func fakeFDTable(t *testing.T, targets map[string]string) string {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("fd discovery relies on POSIX symlink semantics")
	}

	dir := t.TempDir()
	fdDir := filepath.Join(dir, "fd")
	if err := os.MkdirAll(fdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	for fd, target := range targets {
		if err := os.Symlink(target, filepath.Join(fdDir, fd)); err != nil {
			t.Fatalf("symlink fd %s: %v", fd, err)
		}
	}
	return dir
}

func TestExtractJarsFromFDs_FindsOpenJar(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("fd discovery relies on POSIX symlink semantics")
	}

	appDir := t.TempDir()
	jarPath := filepath.Join(appDir, "payments-service.jar")
	if err := os.WriteFile(jarPath, []byte("fat-jar-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	procRoot := fakeFDTable(t, map[string]string{
		"0": "/dev/null",
		"3": jarPath,
		"4": filepath.Join(appDir, "app.log"),
	})

	entries, err := ExtractJarsFromFDs(procRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 jar entry, got %d: %+v", len(entries), entries)
	}
	if entries[0].Path != jarPath {
		t.Errorf("expected %s, got %s", jarPath, entries[0].Path)
	}
	if entries[0].Source != SourceFD {
		t.Errorf("expected source %q, got %q", SourceFD, entries[0].Source)
	}
	if entries[0].Inode == 0 {
		t.Error("fd discovery must record the kernel inode, got 0")
	}
	if entries[0].KernelPath != filepath.Join(procRoot, "fd", "3") {
		t.Errorf("expected the fd handle itself, got %s", entries[0].KernelPath)
	}
}

func TestExtractJarsFromFDs_DeduplicatesByInode(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("fd discovery relies on POSIX symlink semantics")
	}

	appDir := t.TempDir()
	jarPath := filepath.Join(appDir, "service.jar")
	if err := os.WriteFile(jarPath, []byte("bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	// The same jar opened twice must not produce two entries, otherwise the
	// aggregate set digest would depend on descriptor churn.
	procRoot := fakeFDTable(t, map[string]string{"3": jarPath, "7": jarPath})

	entries, err := ExtractJarsFromFDs(procRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 deduplicated entry, got %d", len(entries))
	}
}

func TestExtractJarsFromFDs_NoFDDir(t *testing.T) {
	entries, err := ExtractJarsFromFDs(t.TempDir())
	if err != nil {
		t.Fatalf("a missing fd directory must not be an error: %v", err)
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

	entries, err := ExtractJarsFromCmdline(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Path != "/app/application.jar" {
		t.Errorf("expected /app/application.jar, got %s", entries[0].Path)
	}
	if entries[0].Inode != 0 {
		t.Errorf("cmdline fallback should set Inode=0, got %d", entries[0].Inode)
	}
	if entries[0].Source != SourceCmdline {
		t.Errorf("expected source %q, got %q", SourceCmdline, entries[0].Source)
	}
	if entries[0].KernelPath != "" {
		t.Errorf("cmdline fallback carries no kernel handle, got %s", entries[0].KernelPath)
	}
}

func TestExtractJarsFromCmdline_RelativeJarPath(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("relies on POSIX symlink semantics for /proc/<PID>/cwd")
	}

	dir := t.TempDir()
	cmdline := "java\x00-jar\x00app.jar\x00"
	if err := os.WriteFile(filepath.Join(dir, "cmdline"), []byte(cmdline), 0o644); err != nil {
		t.Fatalf("write cmdline: %v", err)
	}
	if err := os.Symlink("/app", filepath.Join(dir, "cwd")); err != nil {
		t.Fatalf("write cwd symlink: %v", err)
	}

	entries, err := ExtractJarsFromCmdline(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Path != "/app/app.jar" {
		t.Errorf("expected /app/app.jar, got %s", entries[0].Path)
	}
}

func TestExtractJarsFromCmdline_NoJar(t *testing.T) {
	dir := t.TempDir()
	cmdline := "java\x00-cp\x00/app/classes\x00com.example.Main\x00"
	if err := os.WriteFile(filepath.Join(dir, "cmdline"), []byte(cmdline), 0o644); err != nil {
		t.Fatalf("write cmdline: %v", err)
	}

	entries, err := ExtractJarsFromCmdline(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no entries for classpath launch, got %+v", entries)
	}
}
