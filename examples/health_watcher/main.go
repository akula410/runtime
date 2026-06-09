// Package main demonstrates background health monitoring via HealthWatcher.
//
// Run:
//
//	go run ./examples/health_watcher
//
// The health watcher polls service health every second and prints snapshots.
// Press Ctrl+C to stop.
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	runtime "github.com/akula410/runtime"
)

// fluctuatingService randomly alternates between healthy and degraded.
type fluctuatingService struct {
	name    string
	healthy bool
}

func (s *fluctuatingService) Name() string { return s.name }

func (s *fluctuatingService) Start(ctx context.Context) error {
	log.Printf("[%s] started", s.name)
	tick := time.NewTicker(800 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			s.healthy = rand.Intn(2) == 0
		case <-ctx.Done():
			return nil
		}
	}
}

func (s *fluctuatingService) Stop(_ context.Context) error { return nil }

func (s *fluctuatingService) Health(_ context.Context) runtime.HealthStatus {
	msg := "healthy"
	if !s.healthy {
		msg = "degraded"
	}
	return runtime.HealthStatus{Name: s.name, Message: msg}
}

func main() {
	// Health watcher polls every second with a 500ms per-poll timeout.
	hw := runtime.NewHealthWatcher(time.Second, 500*time.Millisecond)

	app, err := runtime.New(
		runtime.WithShutdownTimeout(5*time.Second),
		runtime.WithHealthWatcher(hw),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := app.AddService(&fluctuatingService{name: "worker-1", healthy: true}); err != nil {
		log.Fatal(err)
	}
	if err := app.AddService(&fluctuatingService{name: "worker-2", healthy: true}); err != nil {
		log.Fatal(err)
	}

	// Print health snapshot every 2 seconds.
	go func() {
		for {
			time.Sleep(2 * time.Second)
			snap := hw.Snapshot()
			if snap == nil {
				fmt.Println("health: no data yet")
				continue
			}
			fmt.Println("--- health snapshot ---")
			for _, h := range snap {
				fmt.Printf("  %-15s state=%-10s msg=%s\n", h.Name, h.State, h.Message)
			}
		}
	}()

	fmt.Println("health watcher running — press Ctrl+C to stop")
	if err := app.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
