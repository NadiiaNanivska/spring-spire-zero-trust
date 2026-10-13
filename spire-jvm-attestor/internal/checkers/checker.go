package checkers

import (
	"context"

	"github.com/hashicorp/go-hclog"
	"github.com/yourorg/spire-jvm-attestor/internal/cache"
)

type AttestationContext struct {
	Context context.Context
	PID int32
	ProcRoot string
	HashCache *cache.HashCache
	Logger hclog.Logger
	JDKRoot string
	JARHashes map[string]string
	JARPaths []string
}

type Checker interface {
	Name() string
	Check(ctx *AttestationContext) ([]string, error)
}
