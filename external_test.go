package runtime_test

import (
	"context"
	"os/exec"
	"runtime"
	"testing"
	"time"

	rtime "github.com/akula410/runtime"
)

// shortCommand returns a command that exits immediately with code 0.
func shortCommand() (prog string, args []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", "exit", "0"}
	}
	return "sh", []string{"-c", "exit 0"}
}

// blockingCommand returns a command that blocks until killed.
func blockingCommand() (prog string, args []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", "timeout", "/t", "60", "/nobreak", ">nul"}
	}
	return "sh", []string{"-c", "sleep 60"}
}

func TestExternalProcessServiceCleanExit(t *testing.T) {
	prog, args := shortCommand()
	svc := rtime.NewExternalProcessService("short", prog, args...)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
}

func TestExternalProcessServiceStop(t *testing.T) {
	prog, args := blockingCommand()
	// Verify the command exists before running the test.
	if _, err := exec.LookPath(prog); err != nil {
		t.Skipf("command %q not found: %v", prog, err)
	}

	svc := rtime.NewExternalProcessService("blocker", prog, args...).
		WithKillTimeout(500 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- svc.Start(ctx) }()

	time.Sleep(50 * time.Millisecond)
	cancel() // cancel → interrupt → kill

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start returned unexpected error after cancel: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Start did not return after context cancellation")
	}
}

func TestExternalProcessServiceHealthReportsState(t *testing.T) {
	prog, args := shortCommand()
	svc := rtime.NewExternalProcessService("health-test", prog, args...)

	ctx := context.Background()
	h := svc.Health(ctx)
	if h.Name != "health-test" {
		t.Errorf("health name = %q, want %q", h.Name, "health-test")
	}
}

func TestExternalProcessServiceInvalidCommand(t *testing.T) {
	svc := rtime.NewExternalProcessService("bad", "no-such-binary-xyz-abc-999")
	err := svc.Start(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid command")
	}
}
