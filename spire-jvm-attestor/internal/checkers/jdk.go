package checkers

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yourorg/spire-jvm-attestor/internal/procfs"
)

// JDKChecker reports the version declared by the running executable's JDK release file.
// JAVA_VERSION is a self-reported filesystem fact; it does not cryptographically verify the JDK.
type JDKChecker struct{}

func NewJDKChecker() *JDKChecker { return &JDKChecker{} }
func (c *JDKChecker) Name() string { return "jdk" }

func (c *JDKChecker) Check(ctx *AttestationContext) ([]string, error) {
	exePath, err := os.Readlink(filepath.Join(ctx.ProcRoot, "exe"))
	if err != nil {
		// A missing/inaccessible executable is an unavailable fact, not proof of a bad JVM.
		return nil, nil
	}
	exePath = strings.TrimSuffix(exePath, " (deleted)")
	if filepath.Base(exePath) != "java" {
		// Nonstandard launch layouts are not guessed.
		return nil, nil
	}
	jdkRoot := filepath.Dir(filepath.Dir(exePath))
	releasePath := filepath.Join(ctx.ProcRoot, "root", strings.TrimPrefix(filepath.Join(jdkRoot, "release"), "/"))
	f, err := os.Open(releasePath)
	if err != nil {
		// JREs or custom images without release files yield no version selector.
		return nil, nil
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "JAVA_VERSION=") { continue }
		version := strings.Trim(strings.TrimPrefix(line, "JAVA_VERSION="), "\"")
		if version == "" { return nil, nil }
		return []string{SelectorJavaVersionPrefix + procfs.SanitizeSelector(version)}, nil
	}
	if err := scanner.Err(); err != nil { return nil, fmt.Errorf("read JDK release file: %w", err) }
	return nil, nil
}
