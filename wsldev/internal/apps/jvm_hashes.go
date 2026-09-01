package apps

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"wsldev/internal/kubernetes"
)

// jvmService describes a JVM workload whose jar SHA-256 must be published to the
// jvm-attestor plugin via the jvm-hashes ConfigMap.
type jvmService struct {
	serviceDir  string // repo subdir holding the maven project (e.g. "payments-service")
	jarPattern  string // glob (relative to serviceDir/target) matching the repackaged fat jar
	manifestKey string // in-container jar path == key in jvm-hashes.json

	// Registration metadata: the SPIFFE ID the workload is issued and the k8s
	// ServiceAccount it runs as (used to build the SPIRE registration entry).
	spiffeID       string
	serviceAccount string
}

// jvmServices maps a `wsldev app deploy <arg>` name to its JVM integrity metadata.
// Only these apps are JVM workloads attested by the jvm-attestor plugin.
var jvmServices = map[string]jvmService{
	"payments": {
		serviceDir:     "payments-service",
		jarPattern:     "payments-service-*.jar",
		manifestKey:    "/app/payments-service.jar",
		spiffeID:       "spiffe://zerotrust.lab/ns/spire/sa/payments-app",
		serviceAccount: "payments-sa",
	},
	"orders": {
		serviceDir:     "orders-service",
		jarPattern:     "orders-service-*.jar",
		manifestKey:    "/app/orders-service.jar",
		spiffeID:       "spiffe://zerotrust.lab/ns/spire/sa/orders-app",
		serviceAccount: "orders-sa",
	},
}

const (
	jvmHashesManifestRel = "spiffe-spire/base/jvm-hashes-configmap.yaml"
	spireNamespace       = "spire"
	spireAgentDaemonSet  = "spire-agent"
)

// SyncJVMHashes recomputes the SHA-256 of the freshly built jars for the deployed
// JVM services, rewrites spiffe-spire/base/jvm-hashes-configmap.yaml, applies the
// ConfigMap, and restarts the spire-agent DaemonSet so the plugin reloads the
// manifest (which also clears its in-memory hash cache -> next attestation
// recomputes the hash on the cold path). Non-JVM args are ignored; hashes for
// JVM services that were not deployed this run are preserved as-is.
func SyncJVMHashes(deployed []string) error {
	targets := make([]string, 0, len(deployed))
	for _, name := range deployed {
		if _, ok := jvmServices[name]; ok {
			targets = append(targets, name)
		}
	}
	if len(targets) == 0 {
		return nil
	}

	root, err := resolveRepoRoot()
	if err != nil {
		return err
	}

	manifestPath := filepath.Join(root, filepath.FromSlash(jvmHashesManifestRel))

	jars, err := readExistingHashes(manifestPath)
	if err != nil {
		return err
	}

	for _, name := range targets {
		svc := jvmServices[name]
		jarFile, err := findFatJar(filepath.Join(root, svc.serviceDir, "target"), svc.jarPattern)
		if err != nil {
			return fmt.Errorf("locate jar for %s: %w", name, err)
		}
		hash, err := sha256File(jarFile)
		if err != nil {
			return fmt.Errorf("hash jar for %s: %w", name, err)
		}
		fmt.Printf("jvm-hashes: %s -> %s (%s)\n", svc.manifestKey, hash[:16]+"...", filepath.Base(jarFile))
		jars[svc.manifestKey] = hash
	}

	if err := writeHashesConfigMap(manifestPath, jars); err != nil {
		return err
	}

	if err := kubernetes.Kubectl("apply", "-f", manifestPath); err != nil {
		return fmt.Errorf("apply jvm-hashes ConfigMap: %w", err)
	}

	if err := kubernetes.Kubectl(
		"rollout", "restart", "daemonset/"+spireAgentDaemonSet, "-n", spireNamespace,
	); err != nil {
		return fmt.Errorf("restart spire-agent: %w", err)
	}
	if err := kubernetes.Kubectl(
		"rollout", "status", "daemonset/"+spireAgentDaemonSet, "-n", spireNamespace, "--timeout=180s",
	); err != nil {
		return fmt.Errorf("wait for spire-agent rollout: %w", err)
	}

	fmt.Println("jvm-hashes: ConfigMap applied and spire-agent restarted (hash cache cleared)")
	return nil
}

// findFatJar returns the single repackaged jar matching pattern under dir,
// ignoring maven's -sources/-javadoc/.original side artifacts.
func findFatJar(dir, pattern string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return "", err
	}
	var jars []string
	for _, m := range matches {
		base := filepath.Base(m)
		if strings.HasSuffix(base, "-sources.jar") || strings.HasSuffix(base, "-javadoc.jar") {
			continue
		}
		jars = append(jars, m)
	}
	switch len(jars) {
	case 0:
		return "", fmt.Errorf("no jar matching %q in %s (did `mvn package` run?)", pattern, dir)
	case 1:
		return jars[0], nil
	default:
		return "", fmt.Errorf("ambiguous jars matching %q in %s: %v", pattern, dir, jars)
	}
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type hashManifest struct {
	Version int               `json:"version"`
	Jars    map[string]string `json:"jars"`
}

// readExistingHashes extracts the current jar->hash map from the embedded
// jvm-hashes.json block so hashes for services not deployed this run survive.
// A missing file or unparseable block yields an empty map rather than an error.
func readExistingHashes(manifestPath string) (map[string]string, error) {
	jars := make(map[string]string)

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return jars, nil
		}
		return nil, err
	}

	jsonBlock := extractManifestJSON(string(data))
	if jsonBlock == "" {
		return jars, nil
	}

	var m hashManifest
	if err := json.Unmarshal([]byte(jsonBlock), &m); err != nil {
		return jars, nil
	}
	for k, v := range m.Jars {
		jars[k] = v
	}
	return jars, nil
}

// extractManifestJSON pulls the literal block scalar under the
// `jvm-hashes.json: |` key out of the ConfigMap YAML, de-indenting it back to
// raw JSON. Returns "" if the key is not found.
func extractManifestJSON(content string) string {
	lines := strings.Split(content, "\n")

	start := -1
	keyIndent := 0
	for i, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		if strings.HasPrefix(trimmed, "jvm-hashes.json:") && strings.Contains(ln, "|") {
			start = i + 1
			keyIndent = len(ln) - len(strings.TrimLeft(ln, " "))
			break
		}
	}
	if start == -1 {
		return ""
	}

	var out []string
	blockIndent := -1
	for _, ln := range lines[start:] {
		if strings.TrimSpace(ln) == "" {
			out = append(out, "")
			continue
		}
		indent := len(ln) - len(strings.TrimLeft(ln, " "))
		if indent <= keyIndent {
			break
		}
		if blockIndent == -1 {
			blockIndent = indent
		}
		if len(ln) >= blockIndent {
			out = append(out, ln[blockIndent:])
		} else {
			out = append(out, strings.TrimLeft(ln, " "))
		}
	}
	return strings.Join(out, "\n")
}

// writeHashesConfigMap regenerates the ConfigMap YAML with a deterministic,
// sorted jars map embedded as an indented JSON literal block.
func writeHashesConfigMap(manifestPath string, jars map[string]string) error {
	keys := make([]string, 0, len(jars))
	for k := range jars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("apiVersion: v1\n")
	b.WriteString("kind: ConfigMap\n")
	b.WriteString("metadata:\n")
	b.WriteString("  name: jvm-hashes\n")
	b.WriteString("  namespace: spire\n")
	b.WriteString("data:\n")
	b.WriteString("  # jvm-hashes.json is consumed by the jvm-attestor WorkloadAttestor plugin.\n")
	b.WriteString("  #\n")
	b.WriteString("  # Regenerated automatically by `wsldev app deploy` (SyncJVMHashes) from the\n")
	b.WriteString("  # freshly built target/*.jar. Do not edit by hand; rerun the deploy instead.\n")
	b.WriteString("  jvm-hashes.json: |\n")
	b.WriteString("    {\n")
	b.WriteString("      \"version\": 1,\n")
	b.WriteString("      \"jars\": {\n")
	for i, k := range keys {
		comma := ","
		if i == len(keys)-1 {
			comma = ""
		}
		fmt.Fprintf(&b, "        %q: %q%s\n", k, jars[k], comma)
	}
	b.WriteString("      }\n")
	b.WriteString("    }\n")

	return os.WriteFile(manifestPath, []byte(b.String()), 0o644)
}
