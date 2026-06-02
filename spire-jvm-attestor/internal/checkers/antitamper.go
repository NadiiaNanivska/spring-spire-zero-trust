package checkers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yourorg/spire-jvm-attestor/internal/procfs"
)

var dangerousFlags = []string{
	"-javaagent:",
	"-agentlib:",
	"-agentpath:",
	"-Xrunjdwp:",
	"-Xdebug",
	"-Djdk.attach.allowAttachSelf=true",
	"-Dcom.sun.management.jmxremote",
}

var dangerousEnvVars = []string{
	"JAVA_TOOL_OPTIONS",
	"_JAVA_OPTIONS",
	"JDK_JAVA_OPTIONS",
	"IBM_JAVA_OPTIONS",
}

type AntiTamperChecker struct {
	blockOnAttachSocket bool
}

func NewAntiTamperChecker(blockOnAttachSocket bool) *AntiTamperChecker {
	return &AntiTamperChecker{blockOnAttachSocket: blockOnAttachSocket}
}

func (c *AntiTamperChecker) Name() string {
	return "anti-tamper"
}

func (c *AntiTamperChecker) Check(ctx *AttestationContext) ([]string, error) {
	cmdlineRaw, err := os.ReadFile(filepath.Join(ctx.ProcRoot, "cmdline"))
	if err != nil {
		return nil, fmt.Errorf("cannot read cmdline: %w", err)
	}

	args := strings.Split(string(cmdlineRaw), "\x00")
	for _, arg := range args {
		for _, flag := range dangerousFlags {
			if strings.HasPrefix(arg, flag) {
				return []string{
					SelectorAgentFlagsCleanFalse,
					SelectorSuspiciousFlagPrefix + procfs.SanitizeSelector(flag),
				}, nil
			}
		}
	}

	environRaw, err := os.ReadFile(filepath.Join(ctx.ProcRoot, "environ"))
	if err != nil {
		return nil, fmt.Errorf("cannot read environ: %w", err)
	}

	envMap := procfs.ParseEnviron(string(environRaw))
	for _, key := range dangerousEnvVars {
		if val, exists := envMap[key]; exists && strings.TrimSpace(val) != "" {
			return []string{
				SelectorAgentFlagsCleanFalse,
				SelectorSuspiciousEnvPrefix + key,
			}, nil
		}
	}

	attachSocketPath := filepath.Join(ctx.ProcRoot, "root", "tmp", fmt.Sprintf(".java_pid%d", ctx.PID))
	if _, err := os.Stat(attachSocketPath); err == nil {
		if c.blockOnAttachSocket {
			return nil, fmt.Errorf("JVM Attach API socket exposed at %s; refusing attestation", attachSocketPath)
		}
		return []string{
			SelectorAgentFlagsCleanTrue,
			SelectorAttachSocketExposed,
		}, nil
	}

	return []string{
		SelectorAgentFlagsCleanTrue,
		SelectorAttachSocketClean,
	}, nil
}
