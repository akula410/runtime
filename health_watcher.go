package runtime

import (
	"context"
	"sync"
	"time"
)

// HealthWatcher polls Health() on all services at a regular interval and caches
// the results. Run Watch in a goroutine; cancel its context to stop it.
type HealthWatcher struct {
	Interval time.Duration // poll interval; defaults to 15s when zero
	Timeout  time.Duration // per-poll context timeout; defaults to 5s when zero

	mu       sync.RWMutex
	snapshot []HealthStatus
}

// NewHealthWatcher returns a HealthWatcher configured with the given interval and timeout.
func NewHealthWatcher(interval, timeout time.Duration) *HealthWatcher {
	return &HealthWatcher{Interval: interval, Timeout: timeout}
}

// Watch runs the polling loop and blocks until ctx is cancelled.
func (hw *HealthWatcher) Watch(ctx context.Context, m *Manager) {
	interval := hw.Interval
	if interval <= 0 {
		interval = 15 * time.Second
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			hw.poll(ctx, m)
		}
	}
}

func (hw *HealthWatcher) poll(ctx context.Context, m *Manager) {
	timeout := hw.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	snap := m.Status(pollCtx)
	hw.mu.Lock()
	hw.snapshot = snap
	hw.mu.Unlock()
}

// Snapshot returns a copy of the most recently polled health statuses.
// Returns nil if Watch has not yet completed a single poll.
func (hw *HealthWatcher) Snapshot() []HealthStatus {
	hw.mu.RLock()
	defer hw.mu.RUnlock()
	if hw.snapshot == nil {
		return nil
	}
	out := make([]HealthStatus, len(hw.snapshot))
	copy(out, hw.snapshot)
	return out
}
