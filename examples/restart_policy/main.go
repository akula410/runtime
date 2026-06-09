// Package main demonstrates automatic service restart policies.
//
// Run:
//
//	go run ./examples/restart_policy
//
// The flaky service fails on the first two attempts and then runs normally.
// With RestartOnFailure policy it is automatically restarted up to 3 times.
// Press Ctrl+C to stop.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	runtime "github.com/akula410/runtime"
)

// flakyService fails for the first maxFail calls, then runs until stopped.
type flakyService struct {
	name    string
	maxFail int
	calls   atomic.Int32
}

func (s *flakyService) Name() string { return s.name }

func (s *flakyService) Start(ctx context.Context) error {
	n := int(s.calls.Add(1))
	log.Printf("[%s] Start attempt #%d", s.name, n)
	if n <= s.maxFail {
		time.Sleep(100 * time.Millisecond)
		return errors.New("not ready yet")
	}
	log.Printf("[%s] running successfully", s.name)
	<-ctx.Done()
	return nil
}

func (s *flakyService) Stop(_ context.Context) error { return nil }

func (s *flakyService) Health(_ context.Context) runtime.HealthStatus {
	return runtime.HealthStatus{Name: s.name, Message: fmt.Sprintf("attempt %d", s.calls.Load())}
}

func main() {
	app, err := runtime.New(
		runtime.WithShutdownTimeout(5 * time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}

	svc := &flakyService{name: "flaky-worker", maxFail: 2}

	// Register with RestartOnFailure: restart up to 3 times on error,
	// with a 200ms delay between attempts.
	if err := app.AddServiceWithPolicy(svc, runtime.RestartConfig{
		Policy:      runtime.RestartOnFailure,
		MaxRestarts: 3,
		Delay:       200 * time.Millisecond,
	}); err != nil {
		log.Fatal(err)
	}

	fmt.Println("starting flaky-worker with RestartOnFailure policy...")
	fmt.Println("press Ctrl+C to stop")
	if err := app.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
