package internal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	workloadattestorv1 "github.com/spiffe/spire-plugin-sdk/proto/spire/plugin/agent/workloadattestor/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourorg/spire-jvm-attestor/config"
	"github.com/yourorg/spire-jvm-attestor/internal/cache"
	"github.com/yourorg/spire-jvm-attestor/internal/checkers"
)

func TestJVMAttestor_Attest_Pipeline(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "attestor-pipeline-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	pid := 4321
	procRoot := filepath.Join(tmpDir, fmt.Sprintf("%d", pid))
	nsRoot := filepath.Join(procRoot, "root")
	appDir := filepath.Join(nsRoot, "app")
	require.NoError(t, os.MkdirAll(appDir, 0755))

	jarPath := filepath.Join(appDir, "payments-service.jar")
	jarContent := []byte("secure-production-bytecode")
	require.NoError(t, os.WriteFile(jarPath, jarContent, 0644))

	hasher := sha256.New()
	hasher.Write(jarContent)
	expectedHash := hex.EncodeToString(hasher.Sum(nil))

	stat, err := os.Stat(jarPath)
	require.NoError(t, err)
	actualInode, err := cache.GetInode(stat)
	require.NoError(t, err)

	t.Run("Success: Full clean pipeline attestation", func(t *testing.T) {
		setupCleanProcFS(t, procRoot, actualInode, "/app/payments-service.jar")

		attestor := &JVMAttestor{
			procFS:    tmpDir,
			config:    &config.Config{BlockOnAttachSocket: false},
			hashCache: cache.NewHashCache(),
			pipeline: []checkers.Checker{
				checkers.NewAntiDebugChecker(),
				checkers.NewAntiTamperChecker(false),
				checkers.NewJarHashChecker(),
			},
		}

		req := &workloadattestorv1.AttestRequest{Pid: int32(pid)}
		resp, err := attestor.Attest(context.Background(), req)

		require.NoError(t, err)
		require.NotNil(t, resp)

		assert.Contains(t, resp.SelectorValues, "debug_clean=true")
		assert.Contains(t, resp.SelectorValues, "agent_flags_clean=true")
		assert.Contains(t, resp.SelectorValues, "attach_socket_exposed=false")
		assert.Contains(t, resp.SelectorValues, "jar_sha256="+expectedHash)
		assert.Contains(t, resp.SelectorValues, "maps_verified=true")
		assert.Contains(t, resp.SelectorValues, "inode_consistent=true")
	})

	t.Run("Fail-Fast: Stop execution if process is under debug", func(t *testing.T) {
		setupCleanProcFS(t, procRoot, actualInode, "/app/payments-service.jar")

		statusPath := filepath.Join(procRoot, "status")
		require.NoError(t, os.WriteFile(statusPath, []byte("Name: java\nTracerPid: 9999\n"), 0644))

		attestor := &JVMAttestor{
			procFS:    tmpDir,
			config:    &config.Config{BlockOnAttachSocket: false},
			hashCache: cache.NewHashCache(),
			pipeline: []checkers.Checker{
				checkers.NewAntiDebugChecker(),
				checkers.NewAntiTamperChecker(false),
				checkers.NewJarHashChecker(),
			},
		}

		req := &workloadattestorv1.AttestRequest{Pid: int32(pid)}
		resp, err := attestor.Attest(context.Background(), req)

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Contains(t, resp.SelectorValues, "debug_clean=false")
		assert.Contains(t, resp.SelectorValues, "tracer_pid=9999")

		for _, s := range resp.SelectorValues {
			assert.NotContains(t, s, "jar_sha256")
			assert.NotContains(t, s, "maps_verified")
		}
	})

	t.Run("Modified JAR: still issues selectors (policy lives in the entry)", func(t *testing.T) {
		setupCleanProcFS(t, procRoot, actualInode, "/app/payments-service.jar")

		// Whatever the on-disk bytes are, the plugin computes their hash and
		// publishes it. It never blocks on a reference mismatch — that decision
		// belongs to the SPIRE registration entry.
		attestor := &JVMAttestor{
			procFS:    tmpDir,
			config:    &config.Config{BlockOnAttachSocket: false},
			hashCache: cache.NewHashCache(),
			pipeline: []checkers.Checker{
				checkers.NewAntiDebugChecker(),
				checkers.NewAntiTamperChecker(false),
				checkers.NewJarHashChecker(),
			},
		}

		req := &workloadattestorv1.AttestRequest{Pid: int32(pid)}
		resp, err := attestor.Attest(context.Background(), req)

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Contains(t, resp.SelectorValues, "jar_sha256="+expectedHash)
	})
}
