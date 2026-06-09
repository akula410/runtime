package runtime

import "errors"

var (
	// ErrDuplicateService is returned when a service with the same name is registered twice.
	ErrDuplicateService = errors.New("runtime: service already registered")

	// ErrUnknownService is returned when an operation targets a service that does not exist.
	ErrUnknownService = errors.New("runtime: service not found")

	// ErrStartupFailed is returned when a startup task fails.
	ErrStartupFailed = errors.New("runtime: startup task failed")
)
