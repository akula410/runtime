package runtime

import "context"

// Service is a long-running component managed by the runtime.
type Service interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Health(ctx context.Context) HealthStatus
}
