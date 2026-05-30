package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type AntiDebugChecker struct{}

func NewAntiDebugChecker() *AntiDebugChecker {
	return &AntiDebugChecker{}
}

func (c *AntiDebugChecker) Name() string {
	return "anti-debug"
}

func (c *AntiDebugChecker) Check(ctx *AttestationContext) ([]string, error) {
	statusPath := filepath.Join(ctx.ProcRoot, "status")
	data, err := os.ReadFile(statusPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", statusPath, err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "TracerPid:") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			break
		}

		tracerPid, err := strconv.Atoi(parts[1])
		if err != nil {
			break
		}

		if tracerPid > 0 {
			return []string{
				SelectorDebugCleanFalse,
				SelectorTracerPidPrefix + fmt.Sprintf("%d", tracerPid),
			}, nil
		}
		break
	}

	return []string{SelectorDebugCleanTrue}, nil
}
