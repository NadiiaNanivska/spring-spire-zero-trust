package checkers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLaunchFactsCheckerPublishesIndependentFacts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir,"cmdline"), []byte("java\x00-javaagent:/tmp/agent.jar\x00-Xrunjdwp:transport=dt_socket\x00-jar\x00/app.jar\x00"), 0600); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(dir,"environ"), []byte("JAVA_TOOL_OPTIONS=-Xmx512m\x00"), 0600); err != nil { t.Fatal(err) }
	got, err := NewLaunchFactsChecker(false).Check(&AttestationContext{ProcRoot:dir})
	if err != nil { t.Fatal(err) }
	for _, want := range []string{SelectorHasJavaAgentTrue, SelectorHasJDWPTrue, SelectorJavaToolOptionsSetTrue, SelectorAttachSocketFalse} {
		if !containsSelector(got,want) { t.Errorf("missing %q in %v",want,got) }
	}
}

func TestLaunchFactsCheckerCleanProcess(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir,"cmdline"), []byte("java\x00-jar\x00/app.jar\x00"), 0600); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(dir,"environ"), []byte("PATH=/usr/bin\x00"), 0600); err != nil { t.Fatal(err) }
	got, err := NewLaunchFactsChecker(false).Check(&AttestationContext{ProcRoot:dir})
	if err != nil { t.Fatal(err) }
	for _, want := range []string{SelectorHasJavaAgentFalse, SelectorHasJDWPFalse, SelectorJavaToolOptionsSetFalse, SelectorAttachSocketFalse} {
		if !containsSelector(got,want) { t.Errorf("missing %q in %v",want,got) }
	}
}
