// spire-jvm-attestor/internal/antitamper.go
package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var dangerousFlags = []string{
	"-javaagent:",                       // Java agent — ASM bytecode manipulation
	"-agentlib:",                        // Native JVMTI agent (.so)
	"-agentpath:",                       // Absolute path to native JVMTI agent
	"-Xrunjdwp:",                        // Java Debug Wire Protocol (legacy)
	"-Xdebug",                           // Legacy debug flag (enables JDWP)
	"-Djdk.attach.allowAttachSelf=true", // Permits self-attach via Attach API
	"-Dcom.sun.management.jmxremote",    // JMX remote management (exploitable)
}

var dangerousEnvVars = []string{
	"JAVA_TOOL_OPTIONS", // Standard — supported by all JVM implementations
	"_JAVA_OPTIONS",     // HotSpot-specific
	"JDK_JAVA_OPTIONS",  // JDK 9+
	"IBM_JAVA_OPTIONS",  // IBM J9 / Eclipse OpenJ9
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
	// --- 1: cmdline flags ---
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
					SelectorSuspiciousFlagPrefix + sanitizeSelector(flag),
				}, nil
			}
		}
	}

	// --- 2: environment variables ---
	environRaw, err := os.ReadFile(filepath.Join(ctx.ProcRoot, "environ"))
	if err != nil {
		return nil, fmt.Errorf("cannot read environ: %w", err)
	}

	envMap := parseEnviron(string(environRaw))
	for _, key := range dangerousEnvVars {
		if val, exists := envMap[key]; exists && strings.TrimSpace(val) != "" {
			return []string{
				SelectorAgentFlagsCleanFalse,
				SelectorSuspiciousEnvPrefix + key,
			}, nil
		}
	}

	// --- 3: JVM Attach API socket ---
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
