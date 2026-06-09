# runtime

`github.com/akula410/runtime` — a Go package for managing application lifecycle: configuration loading, startup tasks, long-running services, health tracking, graceful shutdown, and a local HTTP control API.

## When to use

Use this package when your application:
- runs multiple long-lived services (HTTP, TCP, workers, schedulers)
- needs deterministic startup order with dependency checks or migrations
- requires graceful shutdown on SIGINT/SIGTERM
- should be controllable from a separate CLI process without restart

## Installation

```bash
go get github.com/akula410/runtime
```

## Concepts

### StartupTask

A `StartupTask` runs once, sequentially, before any service starts. If a task fails, execution stops and no services are started.

```go
type StartupTask interface {
    Name() string
    Run(ctx context.Context, app *App) error
}
```

Use `StartupTaskFunc` as a quick adapter:

```go
app.AddStartupTask(&runtime.StartupTaskFunc{
    TaskName: "migrate",
    Fn: func(ctx context.Context, a *runtime.App) error {
        return runMigrations(ctx)
    },
})
```

### Service

A `Service` is a long-running component. All registered services start concurrently after startup tasks complete.

```go
type Service interface {
    Name() string
    Start(ctx context.Context) error // blocks until stopped
    Stop(ctx context.Context) error
    Health(ctx context.Context) HealthStatus
}
```

`Start` should block until the service is done (e.g., serve until context is cancelled). `Stop` signals the service to shut down; the manager also cancels the context passed to `Start`.

### ServiceManager

`Manager` tracks service state and provides start/stop/restart/status operations. It is embedded in `App` and accessible via `app.Services()`.

States: `stopped → starting → running → stopping → stopped` (or `failed`).

### App

`App` is the top-level coordinator:

```go
app, err := runtime.New(
    runtime.WithEnv("dev"),
    runtime.WithShutdownTimeout(10 * time.Second),
)
app.AddStartupTask(...)
app.AddService(...)
err = app.Run(ctx) // blocks until shutdown
```

`Run` returns when the app is fully stopped. Call `app.Stop(ctx)` from another goroutine (or via the control API) to trigger graceful shutdown.

## Minimal example

```go
package main

import (
    "context"
    "fmt"
    "log"

    runtime "github.com/akula410/runtime"
)

type myService struct{}

func (s *myService) Name() string { return "my-service" }
func (s *myService) Start(ctx context.Context) error {
    <-ctx.Done()
    return nil
}
func (s *myService) Stop(_ context.Context) error { return nil }
func (s *myService) Health(_ context.Context) runtime.HealthStatus {
    return runtime.HealthStatus{Name: "my-service", Message: "ok"}
}

func main() {
    app, err := runtime.New()
    if err != nil {
        log.Fatal(err)
    }
    app.AddStartupTask(&runtime.StartupTaskFunc{
        TaskName: "check",
        Fn: func(ctx context.Context, _ *runtime.App) error {
            fmt.Println("checking deps...")
            return nil
        },
    })
    if err := app.AddService(&myService{}); err != nil {
        log.Fatal(err)
    }
    if err := app.Run(context.Background()); err != nil {
        log.Fatal(err)
    }
}
```

## Configuration directories

Config is loaded from `<base>/<env>/<filename>` using the `config.Loader` interface. The default loader reads JSON.

```
configs/
  dev/
    app.json
  product/
    app.json
  .local/
    app.json
```

```go
app, _ := runtime.New(
    runtime.WithEnv("dev"),
    runtime.WithConfigBase("configs"),
)
app.AddStartupTask(&runtime.StartupTaskFunc{
    TaskName: "load-config",
    Fn: func(ctx context.Context, a *runtime.App) error {
        return a.LoadConfig(ctx, &myCfg)  // reads configs/dev/app.json
    },
})
```

`LoadConfig` silently skips a missing file — config is optional. Override values in code after loading to implement the priority chain:

```
defaults < config file < env variables < runtime options
```

Use your own loader by implementing `config.Loader`:

```go
type Loader interface {
    Load(ctx context.Context, dir string, target any) error
}
```

## Control server / client

The control server exposes HTTP endpoints on `127.0.0.1` for managing a running process. Register it as a service:

```go
import "github.com/akula410/runtime/control"

ctrlSrv := control.NewServer("127.0.0.1:7070", app, "optional-token")
app.AddService(ctrlSrv)
```

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/status` | App + all services health |
| POST | `/stop` | Graceful app shutdown |
| GET | `/services/{name}/status` | Single service health |
| POST | `/services/{name}/start` | Start a service |
| POST | `/services/{name}/stop` | Stop a service |
| POST | `/services/{name}/restart` | Restart a service |

### Client

```go
client := control.NewClient("http://127.0.0.1:7070", "optional-token")

st, _ := client.Status(ctx)
_ = client.RestartService(ctx, "worker")
_ = client.Stop(ctx)
```

## Graceful shutdown

`Run` listens for SIGINT and SIGTERM (on Linux/macOS/Unix; `os.Interrupt` on Windows). On signal or `app.Stop()`:

1. All services receive a context cancellation.
2. `svc.Stop(ctx)` is called concurrently on each service.
3. The shutdown context has the configured timeout (default 30 s).
4. `Run` returns after all services exit or the timeout expires.

Configure the timeout:

```go
runtime.WithShutdownTimeout(10 * time.Second)
```

## PID file

Prevents duplicate instances:

```go
runtime.WithPIDFile("runtime/app.pid")
```

The file is created on `Run` and removed on clean exit. An existing PID file causes `Run` to return an error immediately.

## Platform compatibility

| Platform | Signals | PID check |
|----------|---------|-----------|
| Linux / macOS / Unix | SIGINT, SIGTERM | `kill -0` via `syscall.Signal(0)` |
| Windows | `os.Interrupt` | `os.FindProcess` |

Platform-specific code uses build tags (`!windows` / `windows`) in `shutdown/` and `process/`.

## Security notes

- The control server binds to `127.0.0.1` by default. **Never bind to `0.0.0.0`** without additional network-level protection (firewall, VPN, mTLS).
- Set a non-empty token to enable bearer-token authentication:
  ```go
  control.NewServer("127.0.0.1:7070", app, "my-secret")
  ```
- Tokens are checked with a direct string comparison — do not log token values.
- The control API can stop or restart services; treat it with the same care as an admin endpoint.

## Examples

- [`examples/basic`](examples/basic/main.go) — startup tasks + mock service
- [`examples/http_tcp`](examples/http_tcp/main.go) — HTTP server + TCP echo server
- [`examples/control`](examples/control/main.go) — control server + client
- [`examples/config`](examples/config/main.go) — per-environment config loading

## API stability

The package is **pre-1.0**. Public interfaces (`StartupTask`, `Service`, `Controller`) are stable. The `Manager` and `App` structs may gain new methods in minor versions.
