package hashsource

import "context"

type HashSource interface {
	GetExpectedHash(ctx context.Context, jarPath string) (string, error)
}