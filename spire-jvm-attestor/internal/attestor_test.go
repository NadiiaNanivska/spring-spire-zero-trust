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
				checkers.NewJDKChecker(),
				checkers.NewLaunchFactsChecker(false),
				checkers.NewJarHashChecker(),
			},
		}

		req := &workloadattestorv1.AttestRequest{Pid: int32(pid)}
		resp, err := attestor.Attest(context.Background(), req)

		require.NoError(t, err)
		require.NotNil(t, resp)

		assert.Contains(t, resp.SelectorValues, "has_javaagent=false")
		assert.Contains(t, resp.SelectorValues, "has_jdwp=false")
		assert.Contains(t, resp.SelectorValues, "attach_socket=false")
		assert.Contains(t, resp.SelectorValues, "jar_sha256="+expectedHash)
		assert.Contains(t, resp.SelectorValues, "maps_verified=true")
		assert.Contains(t, resp.SelectorValues, "inode_consistent=true")
	})


	t.Run("Modified JAR: still issues selectors (policy lives in the entry)", func(t *testing.T) {
		setupCleanProcFS(t, procRoot, actualInode, "/app/payments-service.jar")

		attestor := &JVMAttestor{
			procFS:    tmpDir,
			config:    &config.Config{BlockOnAttachSocket: false},
			hashCache: cache.NewHashCache(),
			pipeline: []checkers.Checker{
				checkers.NewJDKChecker(),
				checkers.NewLaunchFactsChecker(false),
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
