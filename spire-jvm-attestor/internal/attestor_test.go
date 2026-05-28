package internal

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	workloadattestorv1 "github.com/spiffe/spire-plugin-sdk/proto/spire/plugin/agent/workloadattestor/v1"
	"github.com/yourorg/spire-jvm-attestor/config"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ── proc environment builder ──────────────────────────────────────────────────

const testPID = int32(12345)

// procEnv describes a fake /proc/<PID> directory for a test scenario.
type procEnv struct {
	// tracerPid is the value written into /proc/<PID>/status → TracerPid.
	// 0 means no debugger attached (clean).
	tracerPid int

	// cmdlineArgs are the NUL-separated process arguments written to cmdline.
	cmdlineArgs []string

	// envVars are the KEY=VALUE pairs written to environ.
	envVars map[string]string

	// jarContainerPath is the absolute path of the jar inside the container
	// (e.g. /app/service.jar). Written to maps with the correct inode.
	// If empty — no jar in maps or cmdline (triggers the "no jars" error).
	jarContainerPath string

	// jarContent is the bytes written as the jar file.
	// Defaults to fakeJarContent if nil.
	jarContent []byte

	// attachSocket — if true, creates .java_pid<PID> in procRoot/root/tmp/.
	attachSocket bool

	// useSpringBootCmdline — jar appears via -jar flag, not maps.
	useSpringBootCmdline bool
}

// buildFakeProcFS creates a fake /proc/<PID> directory tree in a temp dir
// and returns (procFSRoot, jarSHA256, manifestPath).
//
// procFSRoot is the parent of the per-PID directory, i.e. what you pass to
// newWithProcFS. The per-PID dir itself is procFSRoot/<testPID>.
//
// Each call creates a fresh directory; newWithProcFS creates fresh caches,
// so there is no global state to reset between tests.
func buildFakeProcFS(t *testing.T, env procEnv) (procFSRoot string, jarHash string, manifestPath string) {
	t.Helper()

	fsRoot := t.TempDir()
	pidDir := filepath.Join(fsRoot, fmt.Sprintf("%d", testPID))
	if err := os.MkdirAll(pidDir, 0o755); err != nil {
		t.Fatalf("mkdir pidDir: %v", err)
	}

	// ── /proc/<PID>/status ───────────────────────────────────────────────────
	statusContent := fmt.Sprintf("Name:\tjava\nPid:\t%d\nTracerPid:\t%d\nVmRSS:\t102400 kB\n",
		testPID, env.tracerPid)
	writeFile(t, filepath.Join(pidDir, "status"), statusContent)

	// ── /proc/<PID>/cmdline ──────────────────────────────────────────────────
	args := env.cmdlineArgs
	if len(args) == 0 {
		if env.useSpringBootCmdline && env.jarContainerPath != "" {
			args = []string{"java", "-Xmx512m", "-jar", env.jarContainerPath}
		} else {
			args = []string{"java", "-Xmx512m", "-jar", "/app/service.jar"}
		}
	}
	writeFile(t, filepath.Join(pidDir, "cmdline"), joinNUL(args))

	// ── /proc/<PID>/environ ──────────────────────────────────────────────────
	envMap := env.envVars
	if envMap == nil {
		envMap = map[string]string{"PATH": "/usr/bin"}
	}
	writeFile(t, filepath.Join(pidDir, "environ"), joinEnv(envMap))

	// ── jar file + maps ──────────────────────────────────────────────────────
	jarContent := env.jarContent
	if jarContent == nil {
		jarContent = fakeJarContent
	}

	manifestJars := map[string]string{}
	jarHash = sha256hex(jarContent)

	if env.jarContainerPath != "" {
		jarFSPath := filepath.Join(pidDir, "root", env.jarContainerPath)
		if err := os.MkdirAll(filepath.Dir(jarFSPath), 0o755); err != nil {
			t.Fatalf("mkdir jar parent: %v", err)
		}
		if err := os.WriteFile(jarFSPath, jarContent, 0o644); err != nil {
			t.Fatalf("write jar: %v", err)
		}

		// Get actual inode for the TOCTOU check.
		info, err := os.Stat(jarFSPath)
		if err != nil {
			t.Fatalf("stat jar: %v", err)
		}
		inode := info.Sys().(*syscall.Stat_t).Ino

		// Write maps — include the real inode so the TOCTOU check passes.
		var mapsContent string
		if !env.useSpringBootCmdline {
			mapsContent = fmt.Sprintf(
				"7f0000000000-7f0010000000 r--p 00000000 fd:01 %d %s\n",
				inode, env.jarContainerPath,
			)
		}
		// Empty maps → Spring Boot fallback reads from cmdline.
		writeFile(t, filepath.Join(pidDir, "maps"), mapsContent)

		manifestJars[env.jarContainerPath] = jarHash
	} else {
		// No jar at all — write empty maps and a -cp cmdline (no -jar).
		writeFile(t, filepath.Join(pidDir, "maps"), "")
		writeFile(t, filepath.Join(pidDir, "cmdline"), joinNUL([]string{"java", "-cp", "/app/classes", "com.example.Main"}))
	}

	// ── Attach API socket ────────────────────────────────────────────────────
	if env.attachSocket {
		socketDir := filepath.Join(pidDir, "root", "tmp")
		if err := os.MkdirAll(socketDir, 0o755); err != nil {
			t.Fatalf("mkdir socket dir: %v", err)
		}
		socketPath := filepath.Join(socketDir, fmt.Sprintf(".java_pid%d", testPID))
		writeFile(t, socketPath, "")
	}

	// ── manifest ─────────────────────────────────────────────────────────────
	manifestDir := t.TempDir()
	manifestPath = filepath.Join(manifestDir, "manifest.json")
	if len(manifestJars) > 0 {
		writeManifest(t, manifestPath, manifestJars)
	} else {
		// Write a manifest that lists a different jar (won't match).
		writeManifest(t, manifestPath, map[string]string{"/app/other.jar": "aabb"})
	}

	return fsRoot, jarHash, manifestPath
}

// ── small helpers ─────────────────────────────────────────────────────────────

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", path, err)
	}
}

func joinNUL(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += "\x00"
		}
		out += a
	}
	return out
}

func joinEnv(m map[string]string) string {
	out := ""
	for k, v := range m {
		out += k + "=" + v + "\x00"
	}
	return out
}

func defaultConfig(manifestPath string) *config.Config {
	return &config.Config{
		HashManifestPath:    manifestPath,
		BlockOnAttachSocket: false,
	}
}

func attest(t *testing.T, p *JVMAttestor) (*workloadattestorv1.AttestResponse, error) {
	t.Helper()
	return p.Attest(context.Background(), &workloadattestorv1.AttestRequest{Pid: testPID})
}

// responseContains checks if a selector value appears in the response.
// SPIRE selector format: "key=value" (the "jvm:" prefix is stripped by buildResponse).
func responseContains(resp *workloadattestorv1.AttestResponse, value string) bool {
	for _, v := range resp.SelectorValues {
		if v == value {
			return true
		}
	}
	return false
}

// grpcCode extracts the gRPC status code from an error (or returns codes.OK).
func grpcCode(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	s, ok := status.FromError(err)
	if !ok {
		return codes.Unknown
	}
	return s.Code()
}

// ── integration tests ─────────────────────────────────────────────────────────

// TestAttest_HappyPath exercises the full clean execution path:
// no debugger, no dangerous flags, jar hash matches manifest.
// All three checks pass → full selector set returned.
func TestAttest_HappyPath(t *testing.T) {
	fsRoot, jarHash, manifestPath := buildFakeProcFS(t, procEnv{
		tracerPid:        0,
		jarContainerPath: "/app/service.jar",
	})

	p := newWithProcFS(fsRoot, defaultConfig(manifestPath))
	resp, err := attest(t, p)
	if err != nil {
		t.Fatalf("Attest returned error: %v", err)
	}

	for _, want := range []string{
		"debug_clean=true",
		"agent_flags_clean=true",
		"attach_socket_exposed=false",
		"maps_verified=true",
		"inode_consistent=true",
		"jar_sha256:" + jarHash,
	} {
		if !responseContains(resp, want) {
			t.Errorf("missing selector %q in response %v", want, resp.SelectorValues)
		}
	}
}

// TestAttest_DebuggerAttached — Check 1 fires.
// Expects fail-fast: only debug selectors returned, Checks 2+3 skipped.
func TestAttest_DebuggerAttached(t *testing.T) {
	fsRoot, _, manifestPath := buildFakeProcFS(t, procEnv{
		tracerPid:        9999,
		jarContainerPath: "/app/service.jar",
	})

	p := newWithProcFS(fsRoot, defaultConfig(manifestPath))
	resp, err := attest(t, p)
	if err != nil {
		t.Fatalf("expected response (not error) for debug-detected case: %v", err)
	}

	if !responseContains(resp, "debug_clean=false") {
		t.Errorf("expected debug_clean=false, got %v", resp.SelectorValues)
	}
	if !responseContains(resp, "tracer_pid=9999") {
		t.Errorf("expected tracer_pid=9999, got %v", resp.SelectorValues)
	}

	// Checks 2 and 3 must NOT have run — no agent_flags or jar selectors.
	for _, absent := range []string{"agent_flags_clean=true", "maps_verified=true"} {
		if responseContains(resp, absent) {
			t.Errorf("selector %q must not appear in fail-fast debug response", absent)
		}
	}
}

// TestAttest_JavaAgentFlag — Check 2 fires on -javaagent: in cmdline.
// Expects fail-fast: debug + tamper selectors, Check 3 skipped.
func TestAttest_JavaAgentFlag(t *testing.T) {
	fsRoot, _, manifestPath := buildFakeProcFS(t, procEnv{
		tracerPid: 0,
		cmdlineArgs: []string{
			"java", "-javaagent:/evil/agent.jar", "-jar", "/app/service.jar",
		},
		jarContainerPath: "/app/service.jar",
	})

	p := newWithProcFS(fsRoot, defaultConfig(manifestPath))
	resp, err := attest(t, p)
	if err != nil {
		t.Fatalf("expected response (not error) for tamper-detected case: %v", err)
	}

	if !responseContains(resp, "agent_flags_clean=false") {
		t.Errorf("expected agent_flags_clean=false, got %v", resp.SelectorValues)
	}
	// Check 3 must NOT have run.
	if responseContains(resp, "maps_verified=true") {
		t.Errorf("maps_verified must not appear when Check 2 fails")
	}
}

// TestAttest_DangerousEnvVar — Check 2 fires on JAVA_TOOL_OPTIONS.
func TestAttest_DangerousEnvVar(t *testing.T) {
	fsRoot, _, manifestPath := buildFakeProcFS(t, procEnv{
		tracerPid: 0,
		envVars: map[string]string{
			"JAVA_TOOL_OPTIONS": "-javaagent:/evil.jar",
			"PATH":              "/usr/bin",
		},
		jarContainerPath: "/app/service.jar",
	})

	p := newWithProcFS(fsRoot, defaultConfig(manifestPath))
	resp, err := attest(t, p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !responseContains(resp, "agent_flags_clean=false") {
		t.Errorf("expected agent_flags_clean=false for JAVA_TOOL_OPTIONS, got %v", resp.SelectorValues)
	}
	if !responseContains(resp, "suspicious_env=JAVA_TOOL_OPTIONS") {
		t.Errorf("expected suspicious_env selector, got %v", resp.SelectorValues)
	}
}

// TestAttest_AttachSocket_SelectorMode — socket exists, BlockOnAttachSocket=false.
// Expects attestation to succeed but expose attach_socket_exposed=true selector.
func TestAttest_AttachSocket_SelectorMode(t *testing.T) {
	fsRoot, jarHash, manifestPath := buildFakeProcFS(t, procEnv{
		tracerPid:        0,
		jarContainerPath: "/app/service.jar",
		attachSocket:     true,
	})

	cfg := &config.Config{
		HashManifestPath:    manifestPath,
		BlockOnAttachSocket: false,
	}
	p := newWithProcFS(fsRoot, cfg)
	resp, err := attest(t, p)
	if err != nil {
		t.Fatalf("unexpected error in selector mode: %v", err)
	}

	if !responseContains(resp, "attach_socket_exposed=true") {
		t.Errorf("expected attach_socket_exposed=true, got %v", resp.SelectorValues)
	}
	// Jar hash check should still have run.
	if !responseContains(resp, "jar_sha256:"+jarHash) {
		t.Errorf("expected jar_sha256 selector, got %v", resp.SelectorValues)
	}
}

// TestAttest_AttachSocket_BlockMode — socket exists, BlockOnAttachSocket=true.
// Expects PermissionDenied error — no SVID issued.
func TestAttest_AttachSocket_BlockMode(t *testing.T) {
	fsRoot, _, manifestPath := buildFakeProcFS(t, procEnv{
		tracerPid:        0,
		jarContainerPath: "/app/service.jar",
		attachSocket:     true,
	})

	cfg := &config.Config{
		HashManifestPath:    manifestPath,
		BlockOnAttachSocket: true,
	}
	p := newWithProcFS(fsRoot, cfg)
	_, err := attest(t, p)
	if err == nil {
		t.Fatal("expected error in block mode, got nil")
	}
	if grpcCode(err) != codes.PermissionDenied {
		t.Errorf("expected PermissionDenied, got %v", grpcCode(err))
	}
}

// TestAttest_JarHashMismatch — Check 3 fires because jar on disk differs from manifest.
// Expects PermissionDenied — the most security-critical path.
func TestAttest_JarHashMismatch(t *testing.T) {
	fsRoot, _, manifestPath := buildFakeProcFS(t, procEnv{
		tracerPid:        0,
		jarContainerPath: "/app/service.jar",
	})

	// Overwrite manifest with a wrong hash after procFS is built.
	writeManifest(t, manifestPath, map[string]string{
		"/app/service.jar": "000000000000000000000000000000000000000000000000000000000000dead",
	})

	p := newWithProcFS(fsRoot, defaultConfig(manifestPath))
	_, err := attest(t, p)
	if err == nil {
		t.Fatal("expected error on hash mismatch")
	}
	if grpcCode(err) != codes.PermissionDenied {
		t.Errorf("expected PermissionDenied for hash mismatch, got %v", grpcCode(err))
	}
}

// TestAttest_SpringBoot_HappyPath — jar not in maps, found via -jar in cmdline.
// The Spring Boot fallback path should still verify the hash correctly.
func TestAttest_SpringBoot_HappyPath(t *testing.T) {
	fsRoot, jarHash, manifestPath := buildFakeProcFS(t, procEnv{
		tracerPid:            0,
		jarContainerPath:     "/app/application.jar",
		useSpringBootCmdline: true,
	})

	p := newWithProcFS(fsRoot, defaultConfig(manifestPath))
	resp, err := attest(t, p)
	if err != nil {
		t.Fatalf("Spring Boot happy path failed: %v", err)
	}

	if !responseContains(resp, "maps_verified=true") {
		t.Errorf("expected maps_verified=true, got %v", resp.SelectorValues)
	}
	if !responseContains(resp, "jar_sha256:"+jarHash) {
		t.Errorf("expected jar hash selector, got %v", resp.SelectorValues)
	}
}

// TestAttest_NotConfigured — Attest before Configure returns FailedPrecondition.
func TestAttest_NotConfigured(t *testing.T) {
	p := &JVMAttestor{procFS: "/proc"}
	_, err := p.Attest(context.Background(), &workloadattestorv1.AttestRequest{Pid: 1})
	if err == nil {
		t.Fatal("expected error for unconfigured plugin")
	}
	if grpcCode(err) != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition, got %v", grpcCode(err))
	}
}

// TestAttest_MissingProcDir — PID directory does not exist (process already exited).
// Expects Internal error — SPIRE Agent will retry.
func TestAttest_MissingProcDir(t *testing.T) {
	fsRoot := t.TempDir() // empty — no PID subdir

	p := newWithProcFS(fsRoot, defaultConfig("/tmp/manifest.json"))
	_, err := attest(t, p)
	if err == nil {
		t.Fatal("expected error for missing proc dir")
	}
	if grpcCode(err) != codes.Internal {
		t.Errorf("expected Internal for missing proc dir, got %v", grpcCode(err))
	}
}

// TestAttest_FullSelectorSet verifies the exact set of selectors returned on
// the happy path — regression guard to catch accidental selector renames.
func TestAttest_FullSelectorSet(t *testing.T) {
	fsRoot, jarHash, manifestPath := buildFakeProcFS(t, procEnv{
		tracerPid:        0,
		jarContainerPath: "/app/service.jar",
	})

	p := newWithProcFS(fsRoot, defaultConfig(manifestPath))
	resp, err := attest(t, p)
	if err != nil {
		t.Fatalf("Attest error: %v", err)
	}

	wantSelectors := map[string]bool{
		"debug_clean=true":            false,
		"agent_flags_clean=true":      false,
		"attach_socket_exposed=false": false,
		"maps_verified=true":          false,
		"inode_consistent=true":       false,
		"jar_sha256:" + jarHash:       false,
	}

	for _, v := range resp.SelectorValues {
		if _, ok := wantSelectors[v]; ok {
			wantSelectors[v] = true
		}
	}

	for sel, found := range wantSelectors {
		if !found {
			t.Errorf("expected selector %q missing from response %v", sel, resp.SelectorValues)
		}
	}
}

// TestAttest_XDEBUG_Flag — -Xdebug legacy debug flag triggers Check 2.
func TestAttest_XDEBUG_Flag(t *testing.T) {
	fsRoot, _, manifestPath := buildFakeProcFS(t, procEnv{
		tracerPid:        0,
		cmdlineArgs:      []string{"java", "-Xdebug", "-jar", "/app/service.jar"},
		jarContainerPath: "/app/service.jar",
	})

	p := newWithProcFS(fsRoot, defaultConfig(manifestPath))
	resp, err := attest(t, p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !responseContains(resp, "agent_flags_clean=false") {
		t.Errorf("expected agent_flags_clean=false for -Xdebug, got %v", resp.SelectorValues)
	}
}

// TestAttest_JMX_Flag — -Dcom.sun.management.jmxremote triggers Check 2.
func TestAttest_JMX_Flag(t *testing.T) {
	fsRoot, _, manifestPath := buildFakeProcFS(t, procEnv{
		tracerPid: 0,
		cmdlineArgs: []string{
			"java",
			"-Dcom.sun.management.jmxremote",
			"-jar", "/app/service.jar",
		},
		jarContainerPath: "/app/service.jar",
	})

	p := newWithProcFS(fsRoot, defaultConfig(manifestPath))
	resp, err := attest(t, p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !responseContains(resp, "agent_flags_clean=false") {
		t.Errorf("expected agent_flags_clean=false for JMX flag, got %v", resp.SelectorValues)
	}
}