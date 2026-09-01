package apps

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const sampleConfigMap = `apiVersion: v1
kind: ConfigMap
metadata:
  name: jvm-hashes
  namespace: spire
data:
  # comment line
  #
  # another comment
  jvm-hashes.json: |
    {
      "version": 1,
      "jars": {
        "/app/payments-service.jar": "aaa111",
        "/app/orders-service.jar": "bbb222"
      }
    }
`

func TestExtractManifestJSON(t *testing.T) {
	block := extractManifestJSON(sampleConfigMap)
	if block == "" {
		t.Fatal("expected non-empty JSON block")
	}
	var m hashManifest
	if err := json.Unmarshal([]byte(block), &m); err != nil {
		t.Fatalf("extracted block is not valid JSON: %v\nblock:\n%s", err, block)
	}
	if m.Version != 1 {
		t.Errorf("version = %d, want 1", m.Version)
	}
	if got := m.Jars["/app/payments-service.jar"]; got != "aaa111" {
		t.Errorf("payments hash = %q, want aaa111", got)
	}
	if got := m.Jars["/app/orders-service.jar"]; got != "bbb222" {
		t.Errorf("orders hash = %q, want bbb222", got)
	}
}

func TestExtractManifestJSON_missingKey(t *testing.T) {
	if got := extractManifestJSON("apiVersion: v1\nkind: ConfigMap\n"); got != "" {
		t.Errorf("expected empty result for missing key, got %q", got)
	}
}

func TestWriteAndReadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jvm-hashes-configmap.yaml")

	want := map[string]string{
		"/app/payments-service.jar": "deadbeef",
		"/app/orders-service.jar":   "cafef00d",
	}
	if err := writeHashesConfigMap(path, want); err != nil {
		t.Fatalf("writeHashesConfigMap: %v", err)
	}

	// The embedded block must be valid JSON with the expected jars.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	var m hashManifest
	if err := json.Unmarshal([]byte(extractManifestJSON(string(raw))), &m); err != nil {
		t.Fatalf("regenerated block is not valid JSON: %v\nfile:\n%s", err, raw)
	}

	got, err := readExistingHashes(path)
	if err != nil {
		t.Fatalf("readExistingHashes: %v", err)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("hash for %s = %q, want %q", k, got[k], v)
		}
	}
}

func TestReadExistingHashes_missingFile(t *testing.T) {
	got, err := readExistingHashes(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestSyncJVMHashes_noJVMArgs(t *testing.T) {
	if err := SyncJVMHashes([]string{"payments-sa", "orders-sa"}); err != nil {
		t.Errorf("expected no-op for non-JVM args, got %v", err)
	}
}
