// Package shutdown provides OS signal utilities for graceful shutdown.
package shutdown

import (
	"context"
	"os/signal"
)

// NotifyContext returns a context that is cancelled when an OS termination signal
// is received. The caller must call the returned cancel function to release resources.
// Platform-specific signals are registered in signals_unix.go / signals_windows.go.
func NotifyContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, shutdownSignals...)
}
