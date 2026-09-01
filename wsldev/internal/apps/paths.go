package apps

import (
	"fmt"
	"os"
	"path/filepath"
)

func resolveRepoRoot() (string, error) {
	if p := os.Getenv("REPO_ROOT"); p != "" {
		return validateRepoRoot(p)
	}

	candidates := []string{
		".",
		"..",
	}

	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			cwd,
			filepath.Join(cwd, ".."),
		)
	}

	for _, candidate := range candidates {
		path, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if validated, err := validateRepoRoot(path); err == nil {
			return validated, nil
		}
	}

	return "", fmt.Errorf("repository root not found; set REPO_ROOT to spring-spire-zero-trust")
}

func validateRepoRoot(path string) (string, error) {
	for _, dir := range []string{"payments-service", "orders-service", "spiffe-spire"} {
		if _, err := os.Stat(filepath.Join(path, dir)); err != nil {
			return "", fmt.Errorf("expected %q under %q: %w", dir, path, err)
		}
	}
	return path, nil
}

func serviceDir(name string) (string, error) {
	root, err := resolveRepoRoot()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, name)
	if _, err := os.Stat(filepath.Join(dir, "pom.xml")); err != nil {
		return "", fmt.Errorf("service %q not found at %q: %w", name, dir, err)
	}
	return dir, nil
}

func serviceManifest(serviceDir, manifestFile string) (string, error) {
	path := filepath.Join(serviceDir, manifestFile)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("manifest not found at %q: %w", path, err)
	}
	return path, nil
}
