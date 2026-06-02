package checkers

import "context"
import . "github.com/yourorg/spire-jvm-attestor/internal/cache"

type AttestationContext struct {
	Context   context.Context
	PID       int32
	ProcRoot  string
	HashCache *HashCache
}

type Checker interface {
	Name() string
	Check(ctx *AttestationContext) ([]string, error)
}
