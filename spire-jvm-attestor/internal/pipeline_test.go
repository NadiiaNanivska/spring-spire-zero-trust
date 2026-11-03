package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yourorg/spire-jvm-attestor/config"
)

func pipelineNames(p *JVMAttestor) []string {
	names := make([]string, 0, len(p.pipeline))
	for _, c := range p.pipeline {
		names = append(names, c.Name())
	}
	return names
}

func TestBuildPipeline_RespectsChecks(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
		want []string
	}{
		{"all enabled", config.Config{CheckJDK: true, CheckLaunch: true, CheckJarHash: true, CheckProvenance: true},
			[]string{"jdk", "launch-facts", "jar-hash", "provenance"}},
		{"jdk and launch only", config.Config{CheckJDK: true, CheckLaunch: true},
			[]string{"jdk", "launch-facts"}},
		{"jar-hash only", config.Config{CheckJarHash: true}, []string{"jar-hash"}},
		{"none", config.Config{}, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &JVMAttestor{}
			p.buildPipeline(&tt.cfg)
			assert.Equal(t, tt.want, pipelineNames(p))
		})
	}
}
