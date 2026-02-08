package observability

import (
	"fmt"
	"os"

	"wsldev/internal/kubernetes"
)

const observabilityManifestsDir = "/mnt/c/Users/User/Desktop/LNU/Poc/prometheus"

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

func Deploy() error {
	fmt.Println("Deploying observability tools...")

	if err := os.Chdir(observabilityManifestsDir); err != nil {
		return err
	}

	for _, file := range observabilityManifests {
		if err := kubernetes.Kubectl("apply", "-f", file); err != nil {
			return err
		}
	}

	fmt.Println("Observability tools: Prometheus and Graphana deployed successfully")
	return nil
}
