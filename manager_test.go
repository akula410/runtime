package runtime_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	runtime "github.com/akula410/runtime"
)

// mockService is a controllable Service for testing.
type mockService struct {
	name    string
	startFn func(ctx context.Context) error
	stopFn  func(ctx context.Context) error
}

func (m *mockService) Name() string { return m.name }
func (m *mockService) Start(ctx context.Context) error {
	if m.startFn != nil {
		return m.startFn(ctx)
	}
	<-ctx.Done()
	return nil
}
func (m *mockService) Stop(ctx context.Context) error {
	if m.stopFn != nil {
		return m.stopFn(ctx)
	}
	return nil
}
func (m *mockService) Health(_ context.Context) runtime.HealthStatus {
	return runtime.HealthStatus{Name: m.name}
}

func blockingService(name string) *mockService {
	return &mockService{name: name}
}

func TestManagerRejectsDuplicateNames(t *testing.T) {
	m := runtime.NewManager()
	svc := blockingService("svc")
	if err := m.Register(svc); err != nil {
		t.Fatal(err)
	}
	if err := m.Register(svc); !errors.Is(err, runtime.ErrDuplicateService) {
		t.Fatalf("want ErrDuplicateService, got %v", err)
	}
}

func TestManagerRejectsNilService(t *testing.T) {
	m := runtime.NewManager()
	if err := m.Register(nil); !errors.Is(err, runtime.ErrNilService) {
		t.Fatalf("want ErrNilService, got %v", err)
	}
}

func TestManagerRejectsEmptyName(t *testing.T) {
	m := runtime.NewManager()
	svc := &mockService{name: ""}
	if err := m.Register(svc); !errors.Is(err, runtime.ErrEmptyName) {
		t.Fatalf("want ErrEmptyName, got %v", err)
	}
}

func TestManagerUnknownService(t *testing.T) {
	m := runtime.NewManager()
	if err := m.Stop(context.Background(), "nope"); !errors.Is(err, runtime.ErrUnknownService) {
		t.Fatalf("want ErrUnknownService, got %v", err)
	}
}

func TestManagerStartStopAll(t *testing.T) {
	m := runtime.NewManager()
	for _, name := range []string{"a", "b", "c"} {
		if err := m.Register(blockingService(name)); err != nil {
			t.Fatal(err)
		}
	}

	ctx := context.Background()
	if err := m.StartAll(ctx); err != nil {
		t.Fatal(err)
	}

	time.Sleep(20 * time.Millisecond)

	statuses := m.Status(ctx)
	for _, s := range statuses {
		if s.State != runtime.ServiceRunning {
			t.Errorf("service %q: want Running, got %s", s.Name, s.State)
		}
	}

	stopCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := m.StopAll(stopCtx); err != nil {
		t.Fatalf("StopAll: %v", err)
	}

	for _, s := range m.Status(ctx) {
		if s.State != runtime.ServiceStopped {
			t.Errorf("service %q: want Stopped, got %s", s.Name, s.State)
		}
	}
}

func TestManagerRestartService(t *testing.T) {
	m := runtime.NewManager()
	var startCount atomic.Int32
	svc := &mockService{
		name: "worker",
		startFn: func(ctx context.Context) error {
			startCount.Add(1)
			<-ctx.Done()
			return nil
		},
	}
	if err := m.Register(svc); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := m.Start(ctx, "worker"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)

	stopCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := m.Restart(stopCtx, "worker"); err != nil {
		t.Fatalf("Restart: %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	if startCount.Load() != 2 {
		t.Errorf("startCount = %d, want 2", startCount.Load())
	}

	// cleanup
	stopCtx2, cancel2 := context.WithTimeout(ctx, 2*time.Second)
	defer cancel2()
	_ = m.StopAll(stopCtx2)
}

func TestManagerStatusIsConcurrentSafe(t *testing.T) {
	m := runtime.NewManager()
	for i := range 5 {
		name := "svc" + string(rune('0'+i))
		if err := m.Register(blockingService(name)); err != nil {
			t.Fatal(err)
		}
	}

	ctx := context.Background()
	_ = m.StartAll(ctx)
	time.Sleep(10 * time.Millisecond)

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.Status(ctx)
		}()
	}
	wg.Wait()

	stopCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = m.StopAll(stopCtx)
}

func TestManagerStopIdempotent(t *testing.T) {
	m := runtime.NewManager()
	if err := m.Register(blockingService("svc")); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	// Stop a never-started service — should be a no-op
	if err := m.Stop(ctx, "svc"); err != nil {
		t.Fatalf("Stop on unstarted service: %v", err)
	}
}

func TestManagerServiceFailedState(t *testing.T) {
	m := runtime.NewManager()
	svc := &mockService{
		name:    "broken",
		startFn: func(_ context.Context) error { return errors.New("boom") },
	}
	if err := m.Register(svc); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := m.Start(ctx, "broken"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)

	hs, err := m.StatusOf(ctx, "broken")
	if err != nil {
		t.Fatal(err)
	}
	if hs.State != runtime.ServiceFailed {
		t.Errorf("want Failed, got %s", hs.State)
	}
	if hs.Error == "" {
		t.Error("want non-empty Error field")
	}
}
