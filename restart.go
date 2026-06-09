package runtime

import "time"

// RestartPolicy defines the automatic restart behavior for a service.
type RestartPolicy int

const (
	// RestartNever disables automatic restart (default).
	RestartNever RestartPolicy = iota
	// RestartOnFailure restarts the service only when Start returns a non-nil error.
	RestartOnFailure
	// RestartAlways restarts the service whenever Start exits, regardless of the error.
	RestartAlways
)

// RestartConfig controls automatic restart behaviour for a service.
// Zero value applies RestartNever (no restarts).
type RestartConfig struct {
	Policy      RestartPolicy
	MaxRestarts int           // maximum restart attempts; 0 means unlimited
	Delay       time.Duration // pause between restart attempts
}
