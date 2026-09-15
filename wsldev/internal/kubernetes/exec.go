package kubernetes

import (
	"os"
	"os/exec"
)

func Kubectl(args ...string) error {
	cmd := exec.Command("kubectl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func KubectlOutput(args ...string) (string, error) {
	cmd := exec.Command("kubectl", args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	return string(out), err
}
