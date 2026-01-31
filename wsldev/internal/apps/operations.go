package apps

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"wsldev/internal/kubernetes"
)

func RunCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Deploy(app App) error {
	return kubernetes.Kubectl(
		"apply",
		"-n", app.Namespace,
		"-f", app.Manifests,
	)
}

func Delete(app App) error {
	return kubernetes.Kubectl(
		"delete",
		"-n", app.Namespace,
		"-f", app.Manifests,
	)
}

func Restart(app App) error {
	return kubernetes.Kubectl(
		"rollout", "restart",
		"deployment", app.Name,
		"-n", app.Namespace,
	)
}

func Status(app App) error {
	return kubernetes.Kubectl(
		"get", "pods",
		"-n", app.Namespace,
		"-l", fmt.Sprintf("app=%s", app.Name),
	)
}

func Logs(app App) error {
	return kubernetes.Kubectl(
		"logs",
		"-n", app.Namespace,
		"-l", fmt.Sprintf("app=%s", app.Name),
		"--tail=100",
		"-f",
	)
}

func Exec(app App, cmd []string) error {
	args := []string{
		"exec", "-it",
		"-n", app.Namespace,
		"deployment/" + app.Name,
		"--",
	}
	args = append(args, cmd...)
	return kubernetes.Kubectl(args...)
}

func PortForward(app App, ports string) error {
	parts := strings.Split(ports, ":")
	return kubernetes.Kubectl(
		"port-forward",
		"-n", app.Namespace,
		"deployment/"+app.Name,
		parts[0]+":"+parts[1],
	)
}
