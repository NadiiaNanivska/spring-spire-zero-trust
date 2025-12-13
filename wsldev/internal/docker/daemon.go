package docker

import (
	"errors"
	"os/exec"
	"strings"
	"time"
)

// StartDockerd запускає dockerd у фоні
func StartDockerd() error {
    cmd := exec.Command("sh", "-c", "dockerd > /dev/null 2>&1 &")
    cmd.Start()
    return waitForDocker(30 * time.Second)
}

func waitForDocker(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		ok, _ := IsDockerdRunning()
		if ok {
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return errors.New("docker daemon did not become ready in time")
}

// IsDockerdRunning перевіряє чи працює Docker daemon
func IsDockerdRunning() (bool, error) {
    cmd := exec.Command("docker", "info")
    output, err := cmd.CombinedOutput()
    if err != nil {
        if strings.Contains(string(output), "Is the docker daemon running?") {
            return false, nil
        }
        return false, err
    }
    return true, nil
}
