package observability

import (
	"fmt"
	"os"
	"path/filepath"

	"wsldev/internal/kubernetes"
)

var observabilityManifests = []string{
	"prometheus-configmap.yaml",
	"prometheus-cluster-role.yaml",
	"prometheus-sa.yaml",
	"prometheus-service.yaml",
	"prometheus-deployment.yaml",
	"grafana-datasources.yaml",
	"grafana-dashboards-provisioning.yaml",
	"grafana-dashboards.yaml",
	"grafana.yaml",
}

func resolvePrometheusDir() (string, error) {
	if p := os.Getenv("PROMETHEUS_MANIFESTS_PATH"); p != "" {
		return validatePrometheusDir(p)
	}

	if root := os.Getenv("REPO_ROOT"); root != "" {
		if validated, err := validatePrometheusDir(filepath.Join(root, "prometheus")); err == nil {
			return validated, nil
		}
	}

	candidates := []string{
		"prometheus",
		filepath.Join("..", "prometheus"),
	}

	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(cwd, "prometheus"),
			filepath.Join(cwd, "..", "prometheus"),
		)
	}

	for _, candidate := range candidates {
		path, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if validated, err := validatePrometheusDir(path); err == nil {
			return validated, nil
		}
	}

	return "", fmt.Errorf("prometheus manifests not found; set PROMETHEUS_MANIFESTS_PATH or run from the repo")
}

func validatePrometheusDir(path string) (string, error) {
	marker := filepath.Join(path, "prometheus-deployment.yaml")
	if _, err := os.Stat(marker); err != nil {
		return "", fmt.Errorf("prometheus-deployment.yaml not found in %q: %w", path, err)
	}
	return path, nil
}

func Deploy() error {
	fmt.Println("Deploying observability tools...")

	manifestsDir, err := resolvePrometheusDir()
	if err != nil {
		return err
	}
	fmt.Println("Using manifests:", manifestsDir)

	for _, file := range observabilityManifests {
		manifestPath := filepath.Join(manifestsDir, file)
		if err := kubernetes.Kubectl("apply", "-f", manifestPath); err != nil {
			return err
		}
	}

	fmt.Println("Observability tools: Prometheus and Grafana deployed successfully")
	return nil
}
