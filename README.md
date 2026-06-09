# runtime

`github.com/akula410/runtime` — a Go package for managing application lifecycle: configuration loading, startup tasks, long-running services, health tracking, restart policies, graceful shutdown, and a local HTTP control API.

## When to use

Use this package when your application:
- runs multiple long-lived services (HTTP, TCP, workers, schedulers)
- needs deterministic startup order with dependency checks or migrations
- requires graceful shutdown on SIGINT/SIGTERM
- should be controllable from a separate CLI process without restart
- needs automatic service restart on failure

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

## Restart policies

Services can be automatically restarted when they exit. Register a service with a `RestartConfig`:

```go
app.AddServiceWithPolicy(svc, runtime.RestartConfig{
    Policy:      runtime.RestartOnFailure,
    MaxRestarts: 5,
    Delay:       500 * time.Millisecond,
})
```

### Policies

| Policy | Behaviour |
|--------|-----------|
| `RestartNever` | No automatic restart (default) |
| `RestartOnFailure` | Restart when `Start` returns a non-nil error |
| `RestartAlways` | Restart whenever `Start` exits, success or failure |

### Options

- `MaxRestarts` — maximum number of automatic restarts; `0` means unlimited
- `Delay` — pause between restart attempts

When `MaxRestarts` is exhausted the service enters `failed` state and stops restarting. The restart count is exposed in `HealthStatus.Restarts`.

## Health watcher

`HealthWatcher` polls `Health()` on all services at a configurable interval and caches the results. Use it for background monitoring without blocking the status endpoint.

```go
hw := runtime.NewHealthWatcher(
    15 * time.Second, // poll interval
    5 * time.Second,  // per-poll timeout
)

app, _ := runtime.New(
    runtime.WithHealthWatcher(hw),
)

// Read the latest snapshot from anywhere.
snap := hw.Snapshot()
for _, h := range snap {
    fmt.Printf("%s: %s — %s\n", h.Name, h.State, h.Message)
}
```

The watcher starts automatically when `app.Run` begins and stops before services are shut down.

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
| POST | `/restart` | Restart all services |
| GET | `/services/{name}/status` | Service state + health |
| GET | `/services/{name}/health` | Raw health check from the service |
| POST | `/services/{name}/start` | Start a service |
| POST | `/services/{name}/stop` | Stop a service |
| POST | `/services/{name}/restart` | Restart a service |

### Client

```go
client := control.NewClient("http://127.0.0.1:7070", "optional-token")

st, _  := client.Status(ctx)
_       = client.Restart(ctx)
_       = client.RestartService(ctx, "worker")
hs, _  := client.ServiceHealth(ctx, "worker")
_       = client.Stop(ctx)
```

## External process services

`ExternalProcessService` wraps an OS process as a `Service`. It handles start, graceful stop, and health reporting.

```go
svc := runtime.NewExternalProcessService("redis", "redis-server", "--port", "6379").
    WithKillTimeout(3 * time.Second).
    WithStdout(os.Stdout).
    WithStderr(os.Stderr)

app.AddService(svc)
```

Combine with restart policies for automatic recovery:

```go
app.AddServiceWithPolicy(svc, runtime.RestartConfig{
    Policy:      runtime.RestartOnFailure,
    MaxRestarts: 10,
    Delay:       time.Second,
})
```

### Shutdown sequence

On context cancellation (or `app.Stop()`):

1. SIGINT is sent to the **process group** (Unix) or the process (Windows).
2. If the process does not exit within `KillTimeout`, SIGKILL / `Process.Kill()` is sent.
3. `Start` returns `nil` (clean stop).

### Platform notes

| Platform | Process group | Child processes |
|----------|---------------|-----------------|
| Linux / macOS / Unix | New group via `Setpgid: true`; signals sent with `Kill(-pgid, ...)` | Grandchild processes in the group are also terminated |
| Windows | No process group support (TODO: Job Objects) | Only the direct child receives the signal; grandchildren may remain |

On Windows, if the managed program spawns children (e.g., `cmd.exe` launching another binary), those children are **not** automatically terminated. Use a program that manages its own shutdown, or implement Windows Job Objects for full subtree control.

## Graceful shutdown

`Run` listens for SIGINT and SIGTERM (on Linux/macOS/Unix; `os.Interrupt` on Windows). On signal or `app.Stop()`:

1. Health watcher stops.
2. All services receive a context cancellation.
3. `svc.Stop(ctx)` is called concurrently on each service.
4. The shutdown context has the configured timeout (default 30 s).
5. `Run` returns after all services exit or the timeout expires.
6. PID file is removed (if configured).

Configure the timeout:

```go
runtime.WithShutdownTimeout(10 * time.Second)
```

## PID file

Prevents duplicate instances:

```go
runtime.WithPIDFile("/var/run/myapp.pid")
```

Behaviour:
- Created on `Run` start; removed on clean exit.
- If the file exists and the stored process is alive, `Run` returns an error.
- If the file is stale (process gone) or corrupted, it is removed and recreated automatically.

## Platform compatibility

| Platform | Signals | PID existence check |
|----------|---------|---------------------|
| Linux / macOS / Unix | SIGINT, SIGTERM | `syscall.Signal(0)` (kill -0) |
| Windows | `os.Interrupt` | `OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION)` |

Platform-specific code uses build tags (`!windows` / `windows`) in `shutdown/` and `process/`.

## Security notes

- The control server binds to `127.0.0.1` by default. **Never bind to `0.0.0.0`** without additional network-level protection (firewall, VPN, mTLS).
- Accepted loopback addresses: `127.0.0.1`, `::1`, and `localhost`. Any other host is rejected at startup.
- Set a non-empty token to enable bearer-token authentication:
  ```go
  control.NewServer("127.0.0.1:7070", app, "my-secret")
  ```
- Tokens are checked with direct string comparison — do not log token values.
- The control API can stop or restart services; treat it with the same care as an admin endpoint.

## Examples

- [`examples/basic`](examples/basic/main.go) — startup tasks + mock service
- [`examples/http_tcp`](examples/http_tcp/main.go) — HTTP server + TCP echo server
- [`examples/control`](examples/control/main.go) — control server + client
- [`examples/config`](examples/config/main.go) — per-environment config loading
- [`examples/health_watcher`](examples/health_watcher/main.go) — background health polling
- [`examples/restart_policy`](examples/restart_policy/main.go) — automatic restart on failure
- [`examples/external_process`](examples/external_process/main.go) — managing an external OS process

## API stability

The package is **v0.1 / pre-1.0**. Public interfaces (`StartupTask`, `Service`, `Controller`) are stable. `Manager` and `App` may gain new methods in minor versions.
