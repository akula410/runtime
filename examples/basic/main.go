// Package main demonstrates basic startup tasks and a mock service.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	runtime "github.com/akula410/runtime"
)

// echoService is a minimal Service that logs and waits.
type echoService struct{ name string }

func (s *echoService) Name() string { return s.name }
func (s *echoService) Start(ctx context.Context) error {
	log.Printf("[%s] started", s.name)
	<-ctx.Done()
	log.Printf("[%s] stopped", s.name)
	return nil
}
func (s *echoService) Stop(_ context.Context) error { return nil }
func (s *echoService) Health(_ context.Context) runtime.HealthStatus {
	return runtime.HealthStatus{Name: s.name, Message: "running fine"}
}

func main() {
	app, err := runtime.New(
		runtime.WithShutdownTimeout(5 * time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Startup tasks run sequentially before services start.
	app.AddStartupTask(&runtime.StartupTaskFunc{
		TaskName: "check-deps",
		Fn: func(ctx context.Context, _ *runtime.App) error {
			fmt.Println("task: checking dependencies...")
			return nil
		},
	})
	app.AddStartupTask(&runtime.StartupTaskFunc{
		TaskName: "warm-cache",
		Fn: func(ctx context.Context, _ *runtime.App) error {
			fmt.Println("task: warming cache...")
			return nil
		},
	})

	// Long-running services start concurrently after tasks finish.
	if err := app.AddService(&echoService{"worker-1"}); err != nil {
		log.Fatal(err)
	}
	if err := app.AddService(&echoService{"worker-2"}); err != nil {
		log.Fatal(err)
	}

	fmt.Println("starting... press Ctrl+C to stop")
	if err := app.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
