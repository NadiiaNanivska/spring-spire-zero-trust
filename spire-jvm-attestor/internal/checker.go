package internal

import "context"

type AttestationContext struct {
	Context       context.Context
	PID           int32
	ProcRoot      string
	HashCache     *HashCache
	ManifestCache *ManifestCache
}

type Checker interface {
	Name() string
	Check(ctx *AttestationContext) ([]string, error)
}