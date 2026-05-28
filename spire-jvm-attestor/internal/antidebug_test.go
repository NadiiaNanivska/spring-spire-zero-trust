package internal

import (
	"os"
	"path/filepath"
	"testing"
)

// runAntiDebug is a test helper that wraps AntiDebugChecker.Check with a minimal
// AttestationContext built from procRoot. Mirrors the old checkAntiDebug signature.
func runAntiDebug(t *testing.T, procRoot string) ([]string, error) {
	t.Helper()
	c := NewAntiDebugChecker()
	return c.Check(&AttestationContext{ProcRoot: procRoot})
}

// writeProcStatus creates a fake /proc/<PID>/status file in a temp dir
// and returns the procRoot path.
func writeProcStatus(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "status"), []byte(content), 0o644); err != nil {
		t.Fatalf("write status: %v", err)
	}
	return dir
}

func TestCheckAntiDebug_Clean(t *testing.T) {
	procRoot := writeProcStatus(t, `Name:	java
Pid:	1234
TracerPid:	0
VmRSS:	102400 kB
`)
	selectors, err := runAntiDebug(t, procRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(selectors) != 1 || selectors[0] != "jvm:debug_clean=true" {
		t.Errorf("expected [jvm:debug_clean=true], got %v", selectors)
	}
}

func TestCheckAntiDebug_TracerAttached(t *testing.T) {
	procRoot := writeProcStatus(t, `Name:	java
Pid:	1234
TracerPid:	5678
VmRSS:	102400 kB
`)
	selectors, err := runAntiDebug(t, procRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsSelector(selectors, "jvm:debug_clean=false") {
		t.Errorf("expected jvm:debug_clean=false in selectors, got %v", selectors)
	}
	if !containsSelector(selectors, "jvm:tracer_pid=5678") {
		t.Errorf("expected jvm:tracer_pid=5678 in selectors, got %v", selectors)
	}
}

func TestCheckAntiDebug_MissingStatus(t *testing.T) {
	_, err := runAntiDebug(t, t.TempDir())
	if err == nil {
		t.Error("expected error when status file is missing")
	}
}

func TestCheckAntiDebug_NoTracerPidField(t *testing.T) {
	// Some minimal /proc/status files may omit TracerPid — treat as clean.
	procRoot := writeProcStatus(t, `Name:	java
Pid:	1234
VmRSS:	102400 kB
`)
	selectors, err := runAntiDebug(t, procRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(selectors) != 1 || selectors[0] != "jvm:debug_clean=true" {
		t.Errorf("expected [jvm:debug_clean=true], got %v", selectors)
	}
}