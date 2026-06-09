package runtime

import "context"

// Controller is the interface used by the control server to manage the application.
// App implements this interface.
type Controller interface {
	Status(ctx context.Context) AppStatus
	Shutdown(ctx context.Context) error
	Restart(ctx context.Context) error
	ServiceStatus(ctx context.Context, name string) (HealthStatus, error)
	ServiceHealth(ctx context.Context, name string) (HealthStatus, error)
	StartService(ctx context.Context, name string) error
	StopService(ctx context.Context, name string) error
	RestartService(ctx context.Context, name string) error
}
