package runtime_test

import (
	"context"
	"errors"
	"testing"
	"time"

	runtime "github.com/akula410/runtime"
)

func TestStartupTasksRunSequentially(t *testing.T) {
	var order []string
	app, err := runtime.New()
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"first", "second", "third"} {
		n := name
		app.AddStartupTask(&runtime.StartupTaskFunc{
			TaskName: n,
			Fn: func(_ context.Context, _ *runtime.App) error {
				order = append(order, n)
				return nil
			},
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_ = app.Run(ctx)

	if len(order) != 3 {
		t.Fatalf("want 3 tasks, got %d: %v", len(order), order)
	}
	for i, want := range []string{"first", "second", "third"} {
		if order[i] != want {
			t.Errorf("order[%d] = %q, want %q", i, order[i], want)
		}
	}
}

func TestStartupTaskFailureStopsStartup(t *testing.T) {
	var ran []string
	app, _ := runtime.New()

	app.AddStartupTask(&runtime.StartupTaskFunc{
		TaskName: "ok",
		Fn:       func(_ context.Context, _ *runtime.App) error { ran = append(ran, "ok"); return nil },
	})
	app.AddStartupTask(&runtime.StartupTaskFunc{
		TaskName: "fail",
		Fn:       func(_ context.Context, _ *runtime.App) error { return errors.New("fail") },
	})
	app.AddStartupTask(&runtime.StartupTaskFunc{
		TaskName: "never",
		Fn:       func(_ context.Context, _ *runtime.App) error { ran = append(ran, "never"); return nil },
	})

	ctx := context.Background()
	err := app.Run(ctx)
	if !errors.Is(err, runtime.ErrStartupFailed) {
		t.Fatalf("want ErrStartupFailed, got %v", err)
	}
	for _, r := range ran {
		if r == "never" {
			t.Fatal("task after failure should not run")
		}
	}
}

func TestAppStopSignalsRun(t *testing.T) {
	app, _ := runtime.New(runtime.WithShutdownTimeout(500 * time.Millisecond))
	_ = app.AddService(blockingService("svc"))

	runDone := make(chan error, 1)
	ctx := context.Background()
	go func() { runDone <- app.Run(ctx) }()

	time.Sleep(50 * time.Millisecond)

	if err := app.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after Stop")
	}
}

func TestAppStatus(t *testing.T) {
	app, _ := runtime.New()
	_ = app.AddService(blockingService("svc"))

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- app.Run(ctx) }()

	time.Sleep(50 * time.Millisecond)

	st := app.Status(ctx)
	if len(st.Services) != 1 {
		t.Fatalf("want 1 service, got %d", len(st.Services))
	}

	cancel()
	<-runDone
}
