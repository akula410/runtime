package runtime_test

import (
	"context"
	"testing"
	"time"

	runtime "github.com/akula410/runtime"
)

// TestHealthWatcherSnapshotNilBeforeWatch verifies that Snapshot returns nil
// before Watch has ever been called.
func TestHealthWatcherSnapshotNilBeforeWatch(t *testing.T) {
	hw := runtime.NewHealthWatcher(time.Hour, time.Second)
	if snap := hw.Snapshot(); snap != nil {
		t.Fatalf("expected nil snapshot before Watch is called, got %v", snap)
	}
}

// TestHealthWatcherImmediatePoll verifies the first poll happens immediately
// when Watch starts (no need to wait for the first tick).
func TestHealthWatcherImmediatePoll(t *testing.T) {
	m := runtime.NewManager()
	if err := m.Register(blockingService("svc-a")); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := m.StartAll(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.StopAll(context.Background()) //nolint:errcheck

	time.Sleep(20 * time.Millisecond) // let services reach Running state

	// Use a very long interval so the only poll that fires is the immediate one.
	hw := runtime.NewHealthWatcher(time.Hour, 500*time.Millisecond)

	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go hw.Watch(watchCtx, m)

	// The immediate poll should complete within a few milliseconds.
	deadline := time.Now().Add(500 * time.Millisecond)
	var snap []runtime.HealthStatus
	for time.Now().Before(deadline) {
		snap = hw.Snapshot()
		if snap != nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if snap == nil {
		t.Fatal("snapshot should be populated immediately after Watch starts")
	}
	if len(snap) != 1 || snap[0].Name != "svc-a" {
		t.Fatalf("unexpected snapshot: %v", snap)
	}
}

func TestHealthWatcherPollsAndUpdatesSnapshot(t *testing.T) {
	m := runtime.NewManager()
	if err := m.Register(blockingService("svc-a")); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := m.StartAll(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.StopAll(context.Background()) //nolint:errcheck

	time.Sleep(20 * time.Millisecond)

	hw := runtime.NewHealthWatcher(20*time.Millisecond, 500*time.Millisecond)

	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go hw.Watch(watchCtx, m)

	// With immediate poll + short interval, snapshot should be ready very quickly.
	deadline := time.Now().Add(2 * time.Second)
	var snap []runtime.HealthStatus
	for time.Now().Before(deadline) {
		snap = hw.Snapshot()
		if snap != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if snap == nil {
		t.Fatal("snapshot should be populated after a poll")
	}
	if len(snap) != 1 || snap[0].Name != "svc-a" {
		t.Fatalf("unexpected snapshot: %v", snap)
	}
}

func TestHealthWatcherStopsOnContextCancel(t *testing.T) {
	m := runtime.NewManager()
	hw := runtime.NewHealthWatcher(50*time.Millisecond, 100*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		hw.Watch(ctx, m)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Watch did not stop after context cancellation")
	}
}

func TestHealthWatcherSnapshotIsCopy(t *testing.T) {
	m := runtime.NewManager()
	if err := m.Register(blockingService("svc")); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_ = m.StartAll(ctx)
	defer m.StopAll(context.Background()) //nolint:errcheck

	time.Sleep(20 * time.Millisecond)

	hw := runtime.NewHealthWatcher(time.Hour, 500*time.Millisecond)
	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go hw.Watch(watchCtx, m)

	deadline := time.Now().Add(500 * time.Millisecond)
	var snap []runtime.HealthStatus
	for time.Now().Before(deadline) {
		snap = hw.Snapshot()
		if snap != nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if snap == nil {
		t.Fatal("expected snapshot")
	}

	// Mutating the returned slice must not affect the internal snapshot.
	snap[0].Name = "mutated"
	snap2 := hw.Snapshot()
	if len(snap2) > 0 && snap2[0].Name == "mutated" {
		t.Error("Snapshot should return an independent copy")
	}
}
