package runtime

import "errors"

var (
	// ErrDuplicateService is returned when a service with the same name is registered twice.
	ErrDuplicateService = errors.New("runtime: service already registered")

	// ErrUnknownService is returned when an operation targets a service that does not exist.
	ErrUnknownService = errors.New("runtime: service not found")

	// ErrStartupFailed is returned when a startup task fails.
	ErrStartupFailed = errors.New("runtime: startup task failed")

	// ErrNilService is returned when nil is passed as a Service.
	ErrNilService = errors.New("runtime: service must not be nil")

	// ErrEmptyName is returned when a Service returns an empty name.
	ErrEmptyName = errors.New("runtime: service name must not be empty")
)
