package runtime

import (
	"time"

	"github.com/akula410/runtime/config"
)

// Option configures an App.
type Option func(*App)

// WithEnv sets the environment name used to locate the config directory.
// Combined with WithConfigBase, the effective directory is <base>/<env>.
func WithEnv(env string) Option {
	return func(a *App) { a.env = env }
}

// WithConfigBase sets the base directory that contains per-environment subdirectories.
// Default is "configs".
func WithConfigBase(dir string) Option {
	return func(a *App) { a.configBase = dir }
}

// WithConfigLoader sets the config loader. Defaults to JSONLoader("app.json").
func WithConfigLoader(loader config.Loader) Option {
	return func(a *App) { a.configLoader = loader }
}

// WithShutdownTimeout sets the maximum time allowed for graceful shutdown.
// Default is 30 seconds.
func WithShutdownTimeout(d time.Duration) Option {
	return func(a *App) { a.shutdownTimeout = d }
}

// WithPIDFile sets the path for the PID file. Empty string disables PID file.
func WithPIDFile(path string) Option {
	return func(a *App) { a.pidPath = path }
}
