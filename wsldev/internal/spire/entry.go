package spire

import (
	"encoding/json"
	"fmt"
	"strings"

	"wsldev/internal/kubernetes"
)

const (
	spireServerPod = "spire-server-0"
	spireServerBin = "/opt/spire/bin/spire-server"
)

func EntryCreate(spiffeID, parentID, namespace, serviceAccount string) error {
	cmd := []string{
		"exec", "-n", "spire", spireServerPod, "--",
		spireServerBin, "entry", "create",
		"-spiffeID", spiffeID,
		"-parentID", parentID,
		"-selector", fmt.Sprintf("k8s:ns:%s", namespace),
		"-selector", fmt.Sprintf("k8s:sa:%s", serviceAccount),
	}

	return kubernetes.Kubectl(cmd...)
}

func EntryCreateWithSelectors(spiffeID, parentID string, selectors []string) error {
	cmd := []string{
		"exec", "-n", "spire", spireServerPod, "--",
		spireServerBin, "entry", "create",
		"-spiffeID", spiffeID,
		"-parentID", parentID,
	}
	for _, sel := range selectors {
		cmd = append(cmd, "-selector", sel)
	}
	return kubernetes.Kubectl(cmd...)
}

func GetAgentParentID() (string, error) {
	out, err := kubernetes.KubectlOutput(
		"exec", "-n", "spire", spireServerPod, "--",
		spireServerBin, "agent", "list", "-output", "json",
	)
	if err != nil {
		return "", fmt.Errorf("list spire agents: %w", err)
	}

	// Accept both flat and structured SPIFFE IDs across SPIRE versions.
	var parsed struct {
		Agents []struct {
			SpiffeID string    `json:"spiffe_id"`
			ID       spiffeIDT `json:"id"`
		} `json:"agents"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		return "", fmt.Errorf("parse agent list json: %w", err)
	}
	if len(parsed.Agents) == 0 {
		return "", fmt.Errorf("no attested SPIRE agents found; is the spire-agent DaemonSet running?")
	}
	if id := agentSpiffeID(parsed.Agents[0].SpiffeID, parsed.Agents[0].ID); id != "" {
		return id, nil
	}
	return "", fmt.Errorf("attested SPIRE agent has no resolvable SPIFFE ID")
}

type spiffeIDT struct {
	TrustDomain string `json:"trust_domain"`
	Path        string `json:"path"`
}

func agentSpiffeID(flat string, structured spiffeIDT) string {
	if strings.TrimSpace(flat) != "" {
		return flat
	}
	if strings.TrimSpace(structured.TrustDomain) != "" {
		return fmt.Sprintf("spiffe://%s%s", structured.TrustDomain, structured.Path)
	}
	return ""
}

func EntryDeleteBySpiffeID(spiffeID string) error {
	out, err := kubernetes.KubectlOutput(
		"exec", "-n", "spire", spireServerPod, "--",
		spireServerBin, "entry", "show", "-spiffeID", spiffeID, "-output", "json",
	)
	if err != nil {
		return fmt.Errorf("show entries for %s: %w", spiffeID, err)
	}

	var parsed struct {
		Entries []struct {
			ID      string `json:"id"`
			EntryID string `json:"entry_id"`
		} `json:"entries"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		return fmt.Errorf("parse entry show json: %w", err)
	}

	for _, e := range parsed.Entries {
		entryID := e.ID
		if strings.TrimSpace(entryID) == "" {
			entryID = e.EntryID
		}
		if strings.TrimSpace(entryID) == "" {
			continue
		}
		if err := kubernetes.Kubectl(
			"exec", "-n", "spire", spireServerPod, "--",
			spireServerBin, "entry", "delete", "-entryID", entryID,
		); err != nil {
			return fmt.Errorf("delete entry %s: %w", entryID, err)
		}
	}
	return nil
}

func EntryShow() error {
	return kubernetes.Kubectl(
		"exec", "-n", "spire", "spire-server-0", "--",
		"/opt/spire/bin/spire-server", "entry", "show",
	)
}
