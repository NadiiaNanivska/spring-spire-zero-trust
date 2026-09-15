package kubernetes

import (
	"fmt"
	"os"
	"os/exec"
)

type ClusterManager struct {
	Name string
	Path string
}

func NewClusterManager(name string) *ClusterManager {
	return &ClusterManager{
		Name: name,
		Path: "/usr/local/bin/kind",
	}
}

func (c *ClusterManager) Exists() (bool, error) {
	cmd := exec.Command(c.Path, "get", "clusters")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, err
	}

	clusters := string(output)
	return containsLine(clusters, c.Name), nil
}

func (c *ClusterManager) Create() error {
	exists, err := c.Exists()
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("cluster '%s' already exists", c.Name)
	}

	cmd := exec.Command(c.Path, "create", "cluster", "--name", c.Name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (c *ClusterManager) Delete() error {
	exists, err := c.Exists()
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("cluster '%s' does not exist", c.Name)
	}

	cmd := exec.Command(c.Path, "delete", "cluster", "--name", c.Name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (c *ClusterManager) Reset() error {
	if err := c.Delete(); err != nil {
		fmt.Println("Warning:", err)
	}
	return c.Create()
}

func (c *ClusterManager) Info() error {
	cmd := exec.Command("kubectl", "cluster-info")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func containsLine(output, line string) bool {
	lines := splitLines(output)
	for _, l := range lines {
		if l == line {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var lines []string
	current := ""
	for _, r := range s {
		if r == '\n' || r == '\r' {
			if current != "" {
				lines = append(lines, current)
				current = ""
			}
			continue
		}
		current += string(r)
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
