package apps

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
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

type jvmService struct {
	serviceDir  string
	jarPattern  string
	manifestKey string

	spiffeID       string
	serviceAccount string
}

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
	jvmAgentConfigRel     = "spiffe-spire/base/agent-configmap.yaml"
	spireNamespace       = "spire"
	spireAgentDaemonSet  = "spire-agent"
)

// JVMSigningKeyEnv contains a base64-encoded Ed25519 private key (64 bytes).
// Keep it in a secret store or local environment; never commit the private key.
const JVMSigningKeyEnv = "WSLDEV_JVM_SIGNING_KEY"

// SignJVMJar signs the raw SHA-256 digest and writes the raw Ed25519 signature to <jar>.sig.
func SignJVMJar(jarPath string) (string, error) {
	privateKey, err := jvmSigningPrivateKey()
	if err != nil {
		return "", err
	}
	hashHex, err := sha256File(jarPath)
	if err != nil {
		return "", fmt.Errorf("hash JAR before signing: %w", err)
	}
	digest, err := hex.DecodeString(hashHex)
	if err != nil {
		return "", fmt.Errorf("decode JAR SHA-256: %w", err)
	}
	if err := os.WriteFile(jarPath+".sig", ed25519.Sign(privateKey, digest), 0o644); err != nil {
		return "", fmt.Errorf("write JAR signature: %w", err)
	}
	fingerprint := jvmKeyFingerprint(privateKey.Public().(ed25519.PublicKey))
	fmt.Printf("jvm-signing: %s signed_by=%s\\n", filepath.Base(jarPath), fingerprint)
	return fingerprint, nil
}

func jvmSigningPrivateKey() (ed25519.PrivateKey, error) {
	encoded := strings.TrimSpace(os.Getenv(JVMSigningKeyEnv))
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if encoded == "" || err != nil || len(raw) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("%s must contain a base64-encoded Ed25519 private key (64 bytes)", JVMSigningKeyEnv)
	}
	return ed25519.PrivateKey(raw), nil
}

func jvmKeyFingerprint(publicKey ed25519.PublicKey) string {
	sum := sha256.Sum256(publicKey)
	return "SHA256:" + hex.EncodeToString(sum[:])
}

// JVMKeyPublicConfig returns the trusted public key in the agent plugin's configuration format.
func JVMKeyPublicConfig() (fingerprint, encodedPublicKey string, err error) {
	privateKey, err := jvmSigningPrivateKey()
	if err != nil {
		return "", "", err
	}
	publicKey := privateKey.Public().(ed25519.PublicKey)
	return jvmKeyFingerprint(publicKey), base64.StdEncoding.EncodeToString(publicKey), nil
}

// SyncJVMHashes updates deployed jars and restarts the agent; other hashes are preserved.
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

	if err := syncJVMTrustedPublicKey(root); err != nil {
		return err
	}
	if err := kubernetes.Kubectl("apply", "-f", filepath.Join(root, filepath.FromSlash(jvmAgentConfigRel))); err != nil {
		return fmt.Errorf("apply JVM attestor public key: %w", err)
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

// syncJVMTrustedPublicKey injects the derived public key into the local agent ConfigMap.
func syncJVMTrustedPublicKey(root string) error {
	fingerprint, publicKey, err := JVMKeyPublicConfig()
	if err != nil {
		return err
	}
	path := filepath.Join(root, filepath.FromSlash(jvmAgentConfigRel))
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read SPIRE agent ConfigMap: %w", err)
	}
	content := string(data)
	lines := strings.Split(content, "\n")
	filtered := lines[:0]
	for _, line := range lines {
		if !strings.Contains(line, "public_keys = {") {
			filtered = append(filtered, line)
		}
	}
	content = strings.Join(filtered, "\n")
	old := "          block_on_attach_socket = true"
	newLine := old + "\n          public_keys = { \"" + fingerprint + "\" = \"" + publicKey + "\" }"
	if !strings.Contains(content, old) {
		return fmt.Errorf("cannot find block_on_attach_socket in %s", jvmAgentConfigRel)
	}
	content = strings.Replace(content, old, newLine, 1)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write SPIRE agent ConfigMap: %w", err)
	}
	return nil
}

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
