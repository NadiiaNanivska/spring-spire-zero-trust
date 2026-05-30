package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func runAntiTamper(t *testing.T, procRoot string, pid int32, blockOnAttach bool) ([]string, error) {
	t.Helper()
	c := NewAntiTamperChecker(blockOnAttach)
	return c.Check(&AttestationContext{
		ProcRoot: procRoot,
		PID:      pid,
	})
}

func makeProcDir(t *testing.T, args []string, env map[string]string) string {
	t.Helper()
	dir := t.TempDir()

	cmdline := ""
	for i, a := range args {
		if i > 0 {
			cmdline += "\x00"
		}
		cmdline += a
	}
	if err := os.WriteFile(filepath.Join(dir, "cmdline"), []byte(cmdline), 0o644); err != nil {
		t.Fatalf("write cmdline: %v", err)
	}

	environ := ""
	for k, v := range env {
		environ += k + "=" + v + "\x00"
	}
	if err := os.WriteFile(filepath.Join(dir, "environ"), []byte(environ), 0o644); err != nil {
		t.Fatalf("write environ: %v", err)
	}

	return dir
}

func TestCheckAntiTamper_Clean(t *testing.T) {
	procRoot := makeProcDir(t,
		[]string{"java", "-Xmx512m", "-jar", "/app/service.jar"},
		map[string]string{"PATH": "/usr/bin"},
	)

	selectors, err := runAntiTamper(t, procRoot, 1234, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsSelector(selectors, SelectorAgentFlagsCleanTrue) {
		t.Errorf("expected agent_flags_clean=true, got %v", selectors)
	}
	if !containsSelector(selectors, SelectorAttachSocketClean) {
		t.Errorf("expected attach_socket_exposed=false, got %v", selectors)
	}
}

func TestCheckAntiTamper_JavaAgent(t *testing.T) {
	procRoot := makeProcDir(t,
		[]string{"java", "-javaagent:/evil/agent.jar", "-jar", "/app/service.jar"},
		map[string]string{},
	)

	selectors, err := runAntiTamper(t, procRoot, 1234, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsSelector(selectors, SelectorAgentFlagsCleanFalse) {
		t.Errorf("expected agent_flags_clean=false, got %v", selectors)
	}
}

func TestCheckAntiTamper_JDWP(t *testing.T) {
	procRoot := makeProcDir(t,
		[]string{"java", "-Xrunjdwp:transport=dt_socket,server=y,suspend=n,address=5005", "-jar", "/app/service.jar"},
		map[string]string{},
	)

	selectors, err := runAntiTamper(t, procRoot, 1234, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsSelector(selectors, SelectorAgentFlagsCleanFalse) {
		t.Errorf("expected agent_flags_clean=false for JDWP flag, got %v", selectors)
	}
}

func TestCheckAntiTamper_JavaToolOptions(t *testing.T) {
	procRoot := makeProcDir(t,
		[]string{"java", "-jar", "/app/service.jar"},
		map[string]string{"JAVA_TOOL_OPTIONS": "-javaagent:/evil.jar"},
	)

	selectors, err := runAntiTamper(t, procRoot, 1234, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsSelector(selectors, SelectorAgentFlagsCleanFalse) {
		t.Errorf("expected agent_flags_clean=false for JAVA_TOOL_OPTIONS, got %v", selectors)
	}
	if !containsSelector(selectors, SelectorSuspiciousEnvPrefix+"JAVA_TOOL_OPTIONS") {
		t.Errorf("expected suspicious_env selector, got %v", selectors)
	}
}

func TestCheckAntiTamper_EmptyJavaToolOptions(t *testing.T) {
	// Empty JAVA_TOOL_OPTIONS should be treated as safe.
	procRoot := makeProcDir(t,
		[]string{"java", "-jar", "/app/service.jar"},
		map[string]string{"JAVA_TOOL_OPTIONS": ""},
	)

	selectors, err := runAntiTamper(t, procRoot, 1234, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsSelector(selectors, SelectorAgentFlagsCleanTrue) {
		t.Errorf("empty JAVA_TOOL_OPTIONS should be safe, got %v", selectors)
	}
}

func TestCheckAntiTamper_AttachSocket_BlockMode(t *testing.T) {
	procRoot := makeProcDir(t,
		[]string{"java", "-jar", "/app/service.jar"},
		map[string]string{},
	)

	// Create the Attach API socket file.
	socketDir := filepath.Join(procRoot, "root", "tmp")
	if err := os.MkdirAll(socketDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	socketPath := filepath.Join(socketDir, fmt.Sprintf(".java_pid%d", 1234))
	if err := os.WriteFile(socketPath, []byte{}, 0o600); err != nil {
		t.Fatalf("create socket: %v", err)
	}

	_, err := runAntiTamper(t, procRoot, 1234, true /* blockOnAttachSocket */)
	if err == nil {
		t.Error("expected error in block mode when Attach socket exists")
	}
}

func TestCheckAntiTamper_AttachSocket_SelectorMode(t *testing.T) {
	procRoot := makeProcDir(t,
		[]string{"java", "-jar", "/app/service.jar"},
		map[string]string{},
	)

	socketDir := filepath.Join(procRoot, "root", "tmp")
	if err := os.MkdirAll(socketDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	socketPath := filepath.Join(socketDir, fmt.Sprintf(".java_pid%d", 1234))
	if err := os.WriteFile(socketPath, []byte{}, 0o600); err != nil {
		t.Fatalf("create socket: %v", err)
	}

	selectors, err := runAntiTamper(t, procRoot, 1234, false /* non-blocking */)
	if err != nil {
		t.Fatalf("unexpected error in selector mode: %v", err)
	}
	if !containsSelector(selectors, SelectorAttachSocketExposed) {
		t.Errorf("expected attach_socket_exposed=true, got %v", selectors)
	}
}
