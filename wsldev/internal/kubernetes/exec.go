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

// KubectlOutput runs kubectl and returns its stdout, useful for commands whose
// JSON output must be parsed (e.g. `spire-server agent list -output json`).
// stderr is still forwarded so failures remain visible in the console.
func KubectlOutput(args ...string) (string, error) {
	cmd := exec.Command("kubectl", args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	return string(out), err
}
