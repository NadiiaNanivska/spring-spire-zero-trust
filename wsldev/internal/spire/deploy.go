package spire

import (
	"fmt"
	"os"

	"wsldev/internal/kubernetes"
)

const spireManifestsPath = "/mnt/c/Users/User/Desktop/LNU/Poc/spiffe-spire-quickstart"

var serverManifests = []string{
	"spire-bundle-configmap.yaml",
	"server-account.yaml",
	"server-cluster-role.yaml",
	"server-configmap.yaml",
	"server-service.yaml",
	"server-statefulset.yaml",
}

var agentManifests = []string{
	"agent-account.yaml",
	"agent-cluster-role.yaml",
	"agent-configmap.yaml",
	"agent-daemonset.yaml",
}

func Deploy() error {
	fmt.Println("Deploying SPIRE...")

	_ = kubernetes.Kubectl("create", "namespace", "spire")

	if err := os.Chdir(spireManifestsPath); err != nil {
		return err
	}

	for _, file := range serverManifests {
		if err := kubernetes.Kubectl("apply", "-f", file); err != nil {
			return err
		}
	}

	for _, file := range agentManifests {
		if err := kubernetes.Kubectl("apply", "-f", file); err != nil {
			return err
		}
	}

	fmt.Println("SPIRE deployed successfully")
	return nil
}
