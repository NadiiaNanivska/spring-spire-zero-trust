package checkers

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/yourorg/spire-jvm-attestor/internal/procfs"
)

type LaunchFactsChecker struct { blockOnAttachSocket bool }
func NewLaunchFactsChecker(block bool) *LaunchFactsChecker { return &LaunchFactsChecker{blockOnAttachSocket:block} }
func (c *LaunchFactsChecker) Name() string { return "launch-facts" }

func (c *LaunchFactsChecker) Check(ctx *AttestationContext) ([]string, error) {
	cmdlineRaw, err := os.ReadFile(filepath.Join(ctx.ProcRoot, "cmdline"))
	if err != nil { return nil, err }
	args := strings.Split(string(cmdlineRaw), "\x00")
	hasAgent, hasJDWP := false, false
	for _, arg := range args {
		if strings.HasPrefix(arg, "-javaagent:") || strings.HasPrefix(arg, "-agentlib:") || strings.HasPrefix(arg, "-agentpath:") || strings.HasPrefix(arg, "-Xrun") { hasAgent = true }
		if strings.Contains(arg, "jdwp") || arg == "-Xdebug" { hasJDWP = true }
	}
	envRaw, err := os.ReadFile(filepath.Join(ctx.ProcRoot, "environ"))
	if err != nil { return nil, err }
	env := procfs.ParseEnviron(string(envRaw))
	toolOptions := false
	for _, key := range []string{"JAVA_TOOL_OPTIONS", "_JAVA_OPTIONS", "JDK_JAVA_OPTIONS", "IBM_JAVA_OPTIONS"} {
		if v, ok := env[key]; ok && strings.TrimSpace(v) != "" { toolOptions = true }
	}
	s := []string{SelectorHasJavaAgentFalse, SelectorHasJDWPFalse, SelectorJavaToolOptionsSetFalse}
	if hasAgent { s[0] = SelectorHasJavaAgentTrue; s = append(s, SelectorSuspiciousFlagPrefix+"javaagent") }
	if hasJDWP { s[1] = SelectorHasJDWPTrue; s = append(s, SelectorSuspiciousFlagPrefix+"jdwp") }
	if toolOptions { s[2] = SelectorJavaToolOptionsSetTrue }
	glob := filepath.Join(ctx.ProcRoot, "root", "tmp", ".java_pid*")
	if matches, _ := filepath.Glob(glob); len(matches) > 0 {
		if c.blockOnAttachSocket { return nil, nil } // observation is the default; legacy option remains non-fatal
		s = append(s, SelectorAttachSocket)
	} else { s = append(s, SelectorAttachSocketFalse) }
	return s, nil
}
