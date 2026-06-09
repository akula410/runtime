package runtime

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/akula410/runtime/config"
	"github.com/akula410/runtime/process"
	"github.com/akula410/runtime/shutdown"
)

// App coordinates startup tasks, services, and graceful shutdown.
type App struct {
	manager         *Manager
	tasks           []StartupTask
	env             string
	configBase      string
	configLoader    config.Loader
	shutdownTimeout time.Duration
	pidPath         string

	stopOnce sync.Once
	stopCh   chan struct{}
}

// New creates an App with the given options.
func New(opts ...Option) (*App, error) {
	a := &App{
		manager:         NewManager(),
		configBase:      "configs",
		shutdownTimeout: 30 * time.Second,
		stopCh:          make(chan struct{}),
	}
	for _, o := range opts {
		o(a)
	}
	if a.configLoader == nil {
		a.configLoader = config.NewJSONLoader("app.json")
	}
	return a, nil
}

// AddStartupTask appends a startup task. Tasks run sequentially before services start.
func (a *App) AddStartupTask(task StartupTask) {
	a.tasks = append(a.tasks, task)
}

// AddService registers a service with the manager.
func (a *App) AddService(svc Service) error {
	return a.manager.Register(svc)
}

// Run executes startup tasks, starts services, and blocks until shutdown.
func (a *App) Run(ctx context.Context) error {
	// PID file
	var pidFile *process.PIDFile
	if a.pidPath != "" {
		pf, err := process.CreatePIDFile(a.pidPath)
		if err != nil {
			return fmt.Errorf("runtime: pid file: %w", err)
		}
		pidFile = pf
		defer pidFile.Remove()
	}

	// Startup tasks
	for _, task := range a.tasks {
		if err := task.Run(ctx, a); err != nil {
			return fmt.Errorf("%w: task %q: %v", ErrStartupFailed, task.Name(), err)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}

	// Start services
	if err := a.manager.StartAll(ctx); err != nil {
		return fmt.Errorf("runtime: start services: %w", err)
	}

	// Wait for OS signal or explicit Stop call
	sigCtx, cancelSig := shutdown.NotifyContext(ctx)
	defer cancelSig()

	select {
	case <-sigCtx.Done():
	case <-a.stopCh:
	case <-ctx.Done():
	}

	// Graceful shutdown
	shutCtx, cancelShut := context.WithTimeout(context.Background(), a.shutdownTimeout)
	defer cancelShut()

	return a.manager.StopAll(shutCtx)
}

// Stop signals Run to begin graceful shutdown. It is safe to call concurrently.
func (a *App) Stop(_ context.Context) error {
	a.stopOnce.Do(func() { close(a.stopCh) })
	return nil
}

// Status returns the current health status of all services.
func (a *App) Status(ctx context.Context) AppStatus {
	return AppStatus{Services: a.manager.Status(ctx)}
}

// Env returns the configured environment name (e.g. "dev", ".local").
func (a *App) Env() string { return a.env }

// ConfigDir returns the resolved config directory path (<base>/<env>).
func (a *App) ConfigDir() string {
	if a.env == "" {
		return a.configBase
	}
	return a.configBase + "/" + a.env
}

// LoadConfig loads configuration into target using the configured loader.
func (a *App) LoadConfig(ctx context.Context, target any) error {
	return a.configLoader.Load(ctx, a.ConfigDir(), target)
}

// Services returns the underlying Manager for direct access.
func (a *App) Services() *Manager { return a.manager }

// Controller interface implementation

// Shutdown implements Controller. Signals graceful stop.
func (a *App) Shutdown(ctx context.Context) error { return a.Stop(ctx) }

// ServiceStatus implements Controller.
func (a *App) ServiceStatus(ctx context.Context, name string) (HealthStatus, error) {
	return a.manager.StatusOf(ctx, name)
}

// StartService implements Controller.
func (a *App) StartService(ctx context.Context, name string) error {
	return a.manager.Start(ctx, name)
}

// StopService implements Controller.
func (a *App) StopService(ctx context.Context, name string) error {
	return a.manager.Stop(ctx, name)
}

// RestartService implements Controller.
func (a *App) RestartService(ctx context.Context, name string) error {
	return a.manager.Restart(ctx, name)
}
