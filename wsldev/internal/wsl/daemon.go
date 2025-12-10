package wsl

import (
    "os/exec"
    "strings"
)

// StartDockerd запускає dockerd у фоні
func StartDockerd() error {
    cmd := exec.Command("sh", "-c", "dockerd > /dev/null 2>&1 &")
    return cmd.Start()
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
