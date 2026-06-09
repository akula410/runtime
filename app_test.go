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

func TestStartServiceRequiresRunningApp(t *testing.T) {
	app, _ := runtime.New()
	_ = app.AddService(blockingService("svc"))

	// App is not running yet — StartService must return an error.
	if err := app.StartService(context.Background(), "svc"); err == nil {
		t.Fatal("expected error when app is not running")
	}
}

// TestStartServiceUsesRunContext verifies that a service started via StartService
// is bound to the app's run context, not to the caller's (e.g. HTTP request) context.
func TestStartServiceUsesRunContext(t *testing.T) {
	app, _ := runtime.New(runtime.WithShutdownTimeout(500 * time.Millisecond))
	_ = app.AddService(blockingService("svc"))

	runCtx, cancelRun := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- app.Run(runCtx) }()

	time.Sleep(50 * time.Millisecond)

	// Stop the service so we can re-start it via StartService.
	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := app.Services().Stop(stopCtx, "svc"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	// Use a short-lived request context to start the service.
	reqCtx, cancelReq := context.WithCancel(context.Background())
	if err := app.StartService(reqCtx, "svc"); err != nil {
		t.Fatalf("StartService: %v", err)
	}
	time.Sleep(30 * time.Millisecond)

	// Cancel the request context — the service must remain running.
	cancelReq()
	time.Sleep(50 * time.Millisecond)

	hs, err := app.Services().StatusOf(context.Background(), "svc")
	if err != nil {
		t.Fatalf("StatusOf: %v", err)
	}
	if hs.State == runtime.ServiceStopped || hs.State == runtime.ServiceFailed {
		t.Errorf("service stopped after request context was cancelled (state=%s) — service was incorrectly bound to request context", hs.State)
	}

	cancelRun()
	<-runDone
}
