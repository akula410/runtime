package runtime

import (
	"context"
	"fmt"
	"sync"
)

// managedService wraps a Service with lifecycle state tracking.
type managedService struct {
	svc    Service
	mu     sync.RWMutex
	state  ServiceState
	cancel context.CancelFunc
	done   chan struct{}
	err    error
}

func (ms *managedService) getState() ServiceState {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.state
}

// Manager tracks and controls a set of named services.
type Manager struct {
	mu       sync.RWMutex
	services map[string]*managedService
	order    []string
}

// NewManager creates a new Manager.
func NewManager() *Manager {
	return &Manager{
		services: make(map[string]*managedService),
	}
}

// Register adds a service. Returns ErrDuplicateService if the name is already taken.
func (m *Manager) Register(svc Service) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	name := svc.Name()
	if _, exists := m.services[name]; exists {
		return fmt.Errorf("%w: %q", ErrDuplicateService, name)
	}
	done := make(chan struct{})
	close(done) // closed so any stale wait returns immediately
	m.services[name] = &managedService{
		svc:   svc,
		state: ServiceStopped,
		done:  done,
	}
	m.order = append(m.order, name)
	return nil
}

func (m *Manager) get(name string) (*managedService, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ms, ok := m.services[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownService, name)
	}
	return ms, nil
}

// snapshot returns a stable slice of managed services in registration order.
func (m *Manager) snapshot() []*managedService {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*managedService, 0, len(m.order))
	for _, name := range m.order {
		out = append(out, m.services[name])
	}
	return out
}

// StartAll starts all registered services concurrently.
func (m *Manager) StartAll(ctx context.Context) error {
	for _, ms := range m.snapshot() {
		st := ms.getState()
		if st != ServiceStopped && st != ServiceFailed {
			continue
		}
		m.launch(ctx, ms)
	}
	return nil
}

// StopAll stops all running services concurrently and waits for completion.
func (m *Manager) StopAll(ctx context.Context) error {
	services := m.snapshot()
	errs := make([]error, len(services))
	var wg sync.WaitGroup
	for i, ms := range services {
		if ms.getState() == ServiceStopped {
			continue
		}
		wg.Add(1)
		go func(i int, ms *managedService) {
			defer wg.Done()
			errs[i] = m.halt(ctx, ms)
		}(i, ms)
	}
	wg.Wait()
	return joinErrors(errs)
}

// Start starts the named service. Fails if the service is not stopped/failed.
func (m *Manager) Start(ctx context.Context, name string) error {
	ms, err := m.get(name)
	if err != nil {
		return err
	}
	ms.mu.Lock()
	st := ms.state
	ms.mu.Unlock()
	if st != ServiceStopped && st != ServiceFailed {
		return fmt.Errorf("runtime: service %q is in state %s, cannot start", name, st)
	}
	m.launch(ctx, ms)
	return nil
}

// Stop stops the named service. Idempotent when already stopped.
func (m *Manager) Stop(ctx context.Context, name string) error {
	ms, err := m.get(name)
	if err != nil {
		return err
	}
	if ms.getState() == ServiceStopped {
		return nil
	}
	return m.halt(ctx, ms)
}

// Restart stops then starts the named service.
func (m *Manager) Restart(ctx context.Context, name string) error {
	if err := m.Stop(ctx, name); err != nil {
		return fmt.Errorf("runtime: restart %q stop: %w", name, err)
	}
	return m.Start(ctx, name)
}

// Status returns a health snapshot for all services.
func (m *Manager) Status(ctx context.Context) []HealthStatus {
	services := m.snapshot()
	out := make([]HealthStatus, 0, len(services))
	for _, ms := range services {
		out = append(out, healthOf(ctx, ms))
	}
	return out
}

// StatusOf returns the health status of the named service.
func (m *Manager) StatusOf(ctx context.Context, name string) (HealthStatus, error) {
	ms, err := m.get(name)
	if err != nil {
		return HealthStatus{}, err
	}
	return healthOf(ctx, ms), nil
}

func healthOf(ctx context.Context, ms *managedService) HealthStatus {
	h := ms.svc.Health(ctx)
	ms.mu.RLock()
	h.Name = ms.svc.Name()
	h.State = ms.state
	if ms.err != nil && h.Error == "" {
		h.Error = ms.err.Error()
	}
	ms.mu.RUnlock()
	return h
}

// launch starts ms in a goroutine and transitions state Starting → Running → Stopped/Failed.
func (m *Manager) launch(ctx context.Context, ms *managedService) {
	svcCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	ms.mu.Lock()
	ms.cancel = cancel
	ms.done = done
	ms.err = nil
	ms.state = ServiceStarting
	ms.mu.Unlock()

	go func() {
		defer close(done)

		ms.mu.Lock()
		if ms.state == ServiceStarting {
			ms.state = ServiceRunning
		}
		ms.mu.Unlock()

		err := ms.svc.Start(svcCtx)

		ms.mu.Lock()
		if err != nil && svcCtx.Err() == nil {
			ms.state = ServiceFailed
			ms.err = err
		} else {
			ms.state = ServiceStopped
		}
		ms.mu.Unlock()
	}()
}

// halt cancels the service context, calls svc.Stop, and waits for the goroutine to exit.
func (m *Manager) halt(ctx context.Context, ms *managedService) error {
	ms.mu.Lock()
	ms.state = ServiceStopping
	cancel := ms.cancel
	done := ms.done
	ms.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	stopErr := ms.svc.Stop(ctx)

	select {
	case <-done:
	case <-ctx.Done():
		return fmt.Errorf("runtime: timeout stopping %q: %w", ms.svc.Name(), ctx.Err())
	}

	return stopErr
}

func joinErrors(errs []error) error {
	var first error
	for _, err := range errs {
		if err == nil {
			continue
		}
		if first == nil {
			first = err
		} else {
			first = fmt.Errorf("%w; %v", first, err)
		}
	}
	return first
}
