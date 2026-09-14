package types

import "context"

// Worker is the lifecycle contract implemented by background workers.
type Worker interface {
	Name() string
	Start(ctx context.Context)
}
