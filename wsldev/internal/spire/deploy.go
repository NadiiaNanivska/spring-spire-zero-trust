package spire

import (
	"fmt"
	"os"
	"path/filepath"

	"wsldev/internal/kubernetes"
)

func resolveManifestsPath() (string, error) {
	if p := os.Getenv("SPIRE_MANIFESTS_PATH"); p != "" {
		return validateManifestsDir(p)
	}

	candidates := []string{
		"spiffe-spire",
		filepath.Join("..", "spiffe-spire"),
	}

	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(cwd, "spiffe-spire"),
			filepath.Join(cwd, "..", "spiffe-spire"),
		)
	}

	for _, candidate := range candidates {
		path, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if validated, err := validateManifestsDir(path); err == nil {
			return validated, nil
		}
	}

	return "", fmt.Errorf("spiffe-spire manifests not found; set SPIRE_MANIFESTS_PATH or run from the repo")
}

func validateManifestsDir(path string) (string, error) {
	overlaysDir := filepath.Join(path, "overlays")
	if info, err := os.Stat(overlaysDir); err != nil || !info.IsDir() {
		return "", fmt.Errorf("overlays/ not found in %q: %w", path, err)
	}
	return path, nil
}

// Attestor variants available under spiffe-spire/overlays/.
// "custom-jvm" enables the JVM integrity attestor (anti-debug + anti-tamper +
// jar-hash) alongside the standard k8s/unix attestors; "default" disables it,
// giving a clean baseline for comparative load tests.
const (
	AttestorDefault   = "default"
	AttestorCustomJVM = "custom-jvm"
)

var validAttestorVariants = map[string]bool{
	AttestorDefault:   true,
	AttestorCustomJVM: true,
}

func Deploy(attestorVariant string) error {
	if attestorVariant == "" {
		attestorVariant = AttestorCustomJVM
	}
	if !validAttestorVariants[attestorVariant] {
		return fmt.Errorf("unknown attestor variant %q; valid values: default, custom-jvm", attestorVariant)
	}

	fmt.Printf("Deploying SPIRE (attestor variant: %s)...\n", attestorVariant)

	basePath, err := resolveManifestsPath()
	if err != nil {
		return err
	}
	manifestsPath := filepath.Join(basePath, "overlays", attestorVariant)
	fmt.Println("Using manifests:", manifestsPath)

	if err := kubernetes.Kubectl("apply", "-k", manifestsPath); err != nil {
		return err
	}

	// The SPIRE agent does not hot-reload agent.conf, so pods must restart to
	// pick up a switched attestor variant.
	fmt.Println("Restarting spire-agent DaemonSet to apply config...")
	if err := kubernetes.Kubectl("rollout", "restart", "daemonset/spire-agent", "-n", "spire"); err != nil {
		return err
	}
	if err := kubernetes.Kubectl("rollout", "status", "daemonset/spire-agent", "-n", "spire", "--timeout=120s"); err != nil {
		return err
	}

	fmt.Println("SPIRE deployed successfully")
	return nil
}
