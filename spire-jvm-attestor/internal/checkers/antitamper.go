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

	// The HotSpot Attach API socket is named .java_pid<PID> where PID is the JVM's
	// pid IN ITS OWN namespace (typically 1 or 7 inside a container), NOT the host
	// pid the agent sees as ctx.PID. Matching on the exact ctx.PID name therefore
	// never finds a real containerized attach socket. Glob for any .java_pid* under
	// the workload's /tmp instead.
	attachSocketGlob := filepath.Join(ctx.ProcRoot, "root", "tmp", ".java_pid*")
	if matches, _ := filepath.Glob(attachSocketGlob); len(matches) > 0 {
		if c.blockOnAttachSocket {
			return nil, fmt.Errorf("JVM Attach API socket exposed at %s; refusing attestation", matches[0])
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
