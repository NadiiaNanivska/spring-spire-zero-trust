package internal_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spiffe/spire-plugin-sdk/pluginsdk"
	"github.com/spiffe/spire-plugin-sdk/plugintest"
	workloadattestorv1 "github.com/spiffe/spire-plugin-sdk/proto/spire/plugin/agent/workloadattestor/v1"
	configv1 "github.com/spiffe/spire-plugin-sdk/proto/spire/service/common/config/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/yourorg/spire-jvm-attestor/internal"
)

const integrationTestPID = 9001

// setupFakeProcFS creates a temporary /proc-like directory tree for a fake JVM
// process and a corresponding hash manifest. It returns the proc root, the path
// to the manifest file, and the expected SHA-256 of the fake JAR.
//
// The maps file uses inode=0 so the checker takes the Spring Boot fat-jar path,
// which avoids platform-specific inode behaviour in cross-platform CI.
func setupFakeProcFS(t *testing.T) (procRoot, manifestPath, jarHash string) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "jvm-integration-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	pidRoot := filepath.Join(tmpDir, fmt.Sprintf("%d", integrationTestPID))
	appDir := filepath.Join(pidRoot, "root", "app")
	require.NoError(t, os.MkdirAll(appDir, 0755))

	require.NoError(t, os.WriteFile(
		filepath.Join(pidRoot, "status"),
		[]byte("Name: java\nState: S (sleeping)\nTracerPid: 0\n"),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(pidRoot, "cmdline"),
		[]byte("java\x00-jar\x00/app/payments-service.jar\x00"),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(pidRoot, "environ"),
		[]byte("PATH=/usr/bin\x00HOME=/root\x00"),
		0644,
	))
	// inode=0 → checker treats this as a Spring Boot fat-jar (no inode comparison)
	require.NoError(t, os.WriteFile(
		filepath.Join(pidRoot, "maps"),
		[]byte("00400000-00452000 r-xp 00000000 08:02 0 /app/payments-service.jar\n"),
		0644,
	))

	jarContent := []byte("integration-test-bytecode-payload")
	require.NoError(t, os.WriteFile(filepath.Join(appDir, "payments-service.jar"), jarContent, 0644))
	sum := sha256.Sum256(jarContent)
	jarHash = hex.EncodeToString(sum[:])

	type manifestSchema struct {
		Version int               `json:"version"`
		Jars    map[string]string `json:"jars"`
	}
	raw, err := json.Marshal(manifestSchema{
		Version: 1,
		Jars:    map[string]string{"/app/payments-service.jar": jarHash},
	})
	require.NoError(t, err)

	mf, err := os.CreateTemp("", "jvm-manifest-*.json")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(mf.Name()) })
	_, err = mf.Write(raw)
	require.NoError(t, err)
	require.NoError(t, mf.Close())

	return tmpDir, mf.Name(), jarHash
}

// servePlugin loads the plugin via plugintest (real gRPC transport) and returns
// typed clients. plugintest registers t.Cleanup to shut the server down.
func servePlugin(t *testing.T, plugin *internal.JVMAttestor) (
	*workloadattestorv1.WorkloadAttestorPluginClient,
	*configv1.ConfigServiceClient,
) {
	t.Helper()
	waClient := new(workloadattestorv1.WorkloadAttestorPluginClient)
	cfgClient := new(configv1.ConfigServiceClient)

	plugintest.ServeInBackground(t, plugintest.Config{
		PluginServer: workloadattestorv1.WorkloadAttestorPluginServer(plugin),
		PluginClient: waClient,
		ServiceServers: []pluginsdk.ServiceServer{
			configv1.ConfigServiceServer(plugin),
		},
		ServiceClients: []pluginsdk.ServiceClient{cfgClient},
	})

	return waClient, cfgClient
}

// TestIntegration_NotConfigured verifies that calling Attest before Configure
// returns codes.FailedPrecondition over the gRPC transport.
func TestIntegration_NotConfigured(t *testing.T) {
	plugin := internal.New()
	waClient, _ := servePlugin(t, plugin)

	ctx := context.Background()
	_, err := waClient.Attest(ctx, &workloadattestorv1.AttestRequest{Pid: integrationTestPID})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// TestIntegration_Configure_InvalidHCL verifies that malformed HCL in the
// plugin_data block causes Configure to return codes.InvalidArgument.
func TestIntegration_Configure_InvalidHCL(t *testing.T) {
	plugin := internal.New()
	_, cfgClient := servePlugin(t, plugin)

	ctx := context.Background()
	_, err := cfgClient.Configure(ctx, &configv1.ConfigureRequest{
		HclConfiguration: `= = =`,
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestIntegration_Configure_UnknownSourceType verifies that an unknown
// hash_source_type value causes Configure to return codes.InvalidArgument.
func TestIntegration_Configure_UnknownSourceType(t *testing.T) {
	plugin := internal.New()
	_, cfgClient := servePlugin(t, plugin)

	ctx := context.Background()
	_, err := cfgClient.Configure(ctx, &configv1.ConfigureRequest{
		HclConfiguration: `hash_source_type = "ftp"`,
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestIntegration_Attest_CleanJVM is the end-to-end happy-path test: it
// configures the plugin with a local manifest, then calls Attest over gRPC
// and asserts that the expected selectors are present in the response.
func TestIntegration_Attest_CleanJVM(t *testing.T) {
	procRoot, manifestPath, expectedHash := setupFakeProcFS(t)

	plugin := internal.New()
	plugin.SetProcFSForTest(procRoot)
	waClient, cfgClient := servePlugin(t, plugin)

	ctx := context.Background()
	_, err := cfgClient.Configure(ctx, &configv1.ConfigureRequest{
		HclConfiguration: fmt.Sprintf("hash_manifest_path = %q", manifestPath),
	})
	require.NoError(t, err)

	resp, err := waClient.Attest(ctx, &workloadattestorv1.AttestRequest{Pid: integrationTestPID})
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Contains(t, resp.SelectorValues, "debug_clean=true")
	assert.Contains(t, resp.SelectorValues, "agent_flags_clean=true")
	assert.Contains(t, resp.SelectorValues, "attach_socket_exposed=false")
	assert.Contains(t, resp.SelectorValues, "jar_sha256="+expectedHash)
	assert.Contains(t, resp.SelectorValues, "maps_verified=true")
}

func TestIntegration_Attest_DebuggerAttached(t *testing.T) {
	procRoot, manifestPath, _ := setupFakeProcFS(t)

	statusPath := filepath.Join(procRoot, fmt.Sprintf("%d", integrationTestPID), "status")
	require.NoError(t, os.WriteFile(statusPath, []byte("Name: java\nState: S (sleeping)\nTgid: 4321\nPid: 4321\nTracerPid: 9999\n"), 0644))

	plugin := internal.New()
	plugin.SetProcFSForTest(procRoot)
	waClient, cfgClient := servePlugin(t, plugin)

	ctx := context.Background()
	_, err := cfgClient.Configure(ctx, &configv1.ConfigureRequest{
		HclConfiguration: fmt.Sprintf("hash_manifest_path = %q", manifestPath),
	})
	require.NoError(t, err)

	resp, err := waClient.Attest(ctx, &workloadattestorv1.AttestRequest{Pid: integrationTestPID})
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Contains(t, resp.SelectorValues, "debug_clean=false")
	assert.Contains(t, resp.SelectorValues, "tracer_pid=9999")
	assert.NotContains(t, resp.SelectorValues, "jar_sha256=")
	assert.NotContains(t, resp.SelectorValues, "maps_verified=true")
}

// TestIntegration_Close verifies that Close returns nil and that the plugin
// can be closed multiple times without panicking (idempotence). The manifest
// source holds no network resources, so both calls must be no-ops.
func TestIntegration_Close(t *testing.T) {
	_, manifestPath, _ := setupFakeProcFS(t)

	plugin := internal.New()
	_, cfgClient := servePlugin(t, plugin)

	ctx := context.Background()
	_, err := cfgClient.Configure(ctx, &configv1.ConfigureRequest{
		HclConfiguration: fmt.Sprintf("hash_manifest_path = %q", manifestPath),
	})
	require.NoError(t, err)

	assert.NoError(t, plugin.Close(), "first Close must not error")
	assert.NoError(t, plugin.Close(), "second Close must be idempotent")
}
