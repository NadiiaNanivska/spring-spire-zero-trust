package checkers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestJDKCheckerReadsReleaseVersion(t *testing.T) {
	proc := t.TempDir()
	jdk := filepath.Join(proc,"root","usr","lib","jvm","testjdk")
	if err := os.MkdirAll(filepath.Join(jdk,"bin"),0755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(jdk,"release"), []byte("JAVA_VERSION=\"21.0.5\"\nIMPLEMENTOR=\"Test\"\n"),0644); err != nil { t.Fatal(err) }
	if err := os.Symlink("/usr/lib/jvm/testjdk/bin/java",filepath.Join(proc,"exe")); err != nil { t.Fatal(err) }
	got, err := NewJDKChecker().Check(&AttestationContext{ProcRoot:proc})
	if err != nil { t.Fatal(err) }
	if !containsSelector(got,"jvm:java_version=21.0.5") { t.Fatalf("expected JDK version selector, got %v",got) }
}

func TestJDKCheckerMissingExeIsUnavailableFact(t *testing.T) {
	got, err := NewJDKChecker().Check(&AttestationContext{ProcRoot:t.TempDir()})
	if err != nil { t.Fatalf("missing exe should not fail attestation: %v",err) }
	if len(got) != 0 { t.Fatalf("expected no selectors, got %v",got) }
}
