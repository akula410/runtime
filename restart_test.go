package runtime_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	runtime "github.com/akula410/runtime"
)

// failingService returns an error from Start on every call.
type failingService struct {
	name  string
	calls atomic.Int32
}

func (s *failingService) Name() string { return s.name }
func (s *failingService) Start(_ context.Context) error {
	s.calls.Add(1)
	return errors.New("service boom")
}
func (s *failingService) Stop(_ context.Context) error { return nil }
func (s *failingService) Health(_ context.Context) runtime.HealthStatus {
	return runtime.HealthStatus{Name: s.name}
}

// exitingService exits cleanly (returns nil from Start).
type exitingService struct {
	name  string
	calls atomic.Int32
}

func (s *exitingService) Name() string { return s.name }
func (s *exitingService) Start(_ context.Context) error {
	s.calls.Add(1)
	return nil
}
func (s *exitingService) Stop(_ context.Context) error { return nil }
func (s *exitingService) Health(_ context.Context) runtime.HealthStatus {
	return runtime.HealthStatus{Name: s.name}
}

func TestRestartNeverOnFailure(t *testing.T) {
	m := runtime.NewManager()
	svc := &failingService{name: "broken"}
	if err := m.RegisterWithPolicy(svc, runtime.RestartConfig{Policy: runtime.RestartNever}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := m.Start(ctx, "broken"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)

	if calls := svc.calls.Load(); calls != 1 {
		t.Errorf("expected 1 Start call, got %d", calls)
	}
	hs, _ := m.StatusOf(ctx, "broken")
	if hs.State != runtime.ServiceFailed {
		t.Errorf("expected Failed, got %s", hs.State)
	}
}

func TestRestartOnFailure(t *testing.T) {
	m := runtime.NewManager()
	svc := &failingService{name: "flaky"}
	if err := m.RegisterWithPolicy(svc, runtime.RestartConfig{
		Policy:      runtime.RestartOnFailure,
		MaxRestarts: 2,
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := m.Start(ctx, "flaky"); err != nil {
		t.Fatal(err)
	}
	// 1 initial + 2 restarts = 3 total calls
	time.Sleep(200 * time.Millisecond)

	if calls := svc.calls.Load(); calls != 3 {
		t.Errorf("expected 3 Start calls, got %d", calls)
	}
	hs, _ := m.StatusOf(ctx, "flaky")
	if hs.State != runtime.ServiceFailed {
		t.Errorf("expected Failed after exhausting restarts, got %s", hs.State)
	}
	if hs.Restarts != 2 {
		t.Errorf("expected 2 restarts in status, got %d", hs.Restarts)
	}
}

func TestRestartAlwaysOnCleanExit(t *testing.T) {
	m := runtime.NewManager()
	svc := &exitingService{name: "short-lived"}
	if err := m.RegisterWithPolicy(svc, runtime.RestartConfig{
		Policy:      runtime.RestartAlways,
		MaxRestarts: 3,
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := m.Start(ctx, "short-lived"); err != nil {
		t.Fatal(err)
	}
	// 1 initial + 3 restarts = 4 total calls
	time.Sleep(200 * time.Millisecond)

	if calls := svc.calls.Load(); calls != 4 {
		t.Errorf("expected 4 Start calls, got %d", calls)
	}
	hs, _ := m.StatusOf(ctx, "short-lived")
	// Clean exit exhausted → stopped, not failed
	if hs.State != runtime.ServiceStopped {
		t.Errorf("expected Stopped after clean-exit restarts, got %s", hs.State)
	}
}

func TestRestartOnFailureDoesNotRestartCleanExit(t *testing.T) {
	m := runtime.NewManager()
	svc := &exitingService{name: "clean"}
	if err := m.RegisterWithPolicy(svc, runtime.RestartConfig{
		Policy:      runtime.RestartOnFailure,
		MaxRestarts: 5,
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := m.Start(ctx, "clean"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)

	// Clean exit with RestartOnFailure must NOT restart.
	if calls := svc.calls.Load(); calls != 1 {
		t.Errorf("RestartOnFailure must not restart on clean exit; got %d calls", calls)
	}
	hs, _ := m.StatusOf(ctx, "clean")
	if hs.State != runtime.ServiceStopped {
		t.Errorf("expected Stopped, got %s", hs.State)
	}
}

func TestRestartWithDelay(t *testing.T) {
	m := runtime.NewManager()
	svc := &failingService{name: "delayed"}
	delay := 50 * time.Millisecond
	if err := m.RegisterWithPolicy(svc, runtime.RestartConfig{
		Policy:      runtime.RestartOnFailure,
		MaxRestarts: 2,
		Delay:       delay,
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	start := time.Now()
	if err := m.Start(ctx, "delayed"); err != nil {
		t.Fatal(err)
	}

	// 3 total calls with 2 delays of 50ms each → ≥100ms total
	time.Sleep(500 * time.Millisecond)

	elapsed := time.Since(start)
	if calls := svc.calls.Load(); calls != 3 {
		t.Errorf("expected 3 Start calls, got %d", calls)
	}
	if elapsed < 2*delay {
		t.Errorf("expected at least %v elapsed with delays, got %v", 2*delay, elapsed)
	}
}

func TestRestartUnlimitedUntilStopped(t *testing.T) {
	m := runtime.NewManager()
	svc := &failingService{name: "infinite"}
	if err := m.RegisterWithPolicy(svc, runtime.RestartConfig{
		Policy:      runtime.RestartOnFailure,
		MaxRestarts: 0, // unlimited
		Delay:       5 * time.Millisecond,
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := m.Start(ctx, "infinite"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)

	stopCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := m.Stop(stopCtx, "infinite"); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if calls := svc.calls.Load(); calls < 3 {
		t.Errorf("expected multiple restarts, got %d calls", calls)
	}
	hs, _ := m.StatusOf(ctx, "infinite")
	if hs.State != runtime.ServiceStopped {
		t.Errorf("expected Stopped after manual stop, got %s", hs.State)
	}
}

func TestManagerRestartAll(t *testing.T) {
	m := runtime.NewManager()
	var countA, countB atomic.Int32

	makeBlocking := func(name string, calls *atomic.Int32) *mockService {
		return &mockService{
			name: name,
			startFn: func(ctx context.Context) error {
				calls.Add(1)
				<-ctx.Done()
				return nil
			},
		}
	}

	_ = m.Register(makeBlocking("a", &countA))
	_ = m.Register(makeBlocking("b", &countB))

	ctx := context.Background()
	_ = m.StartAll(ctx)
	time.Sleep(30 * time.Millisecond)

	stopCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := m.RestartAll(stopCtx); err != nil {
		t.Fatalf("RestartAll: %v", err)
	}
	time.Sleep(30 * time.Millisecond)

	if countA.Load() != 2 {
		t.Errorf("service a: expected 2 starts, got %d", countA.Load())
	}
	if countB.Load() != 2 {
		t.Errorf("service b: expected 2 starts, got %d", countB.Load())
	}

	_ = m.StopAll(stopCtx)
}
