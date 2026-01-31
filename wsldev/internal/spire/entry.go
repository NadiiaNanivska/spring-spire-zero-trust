package spire

import (
	"fmt"

	"wsldev/internal/kubernetes"
)

func EntryCreate(spiffeID, parentID, namespace, serviceAccount string) error {
	cmd := []string{
		"exec", "-n", "spire", "spire-server-0", "--",
		"/opt/spire/bin/spire-server", "entry", "create",
		"-spiffeID", spiffeID,
		"-parentID", parentID,
		"-selector", fmt.Sprintf("k8s:ns:%s", namespace),
		"-selector", fmt.Sprintf("k8s:sa:%s", serviceAccount),
	}

	return kubernetes.Kubectl(cmd...)
}

func EntryShow() error {
	return kubernetes.Kubectl(
		"exec", "-n", "spire", "spire-server-0", "--",
		"/opt/spire/bin/spire-server", "entry", "show",
	)
}
