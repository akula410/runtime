package process_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/akula410/runtime/process"
)

func TestPIDFileCreateRemove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.pid")

	pf, err := process.CreatePIDFile(path)
	if err != nil {
		t.Fatalf("CreatePIDFile: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("pid file should exist: %v", err)
	}

	if err := pf.Remove(); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("pid file should be removed")
	}
}

func TestPIDFileDuplicate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.pid")

	pf, err := process.CreatePIDFile(path)
	if err != nil {
		t.Fatalf("first CreatePIDFile: %v", err)
	}
	defer pf.Remove()

	// Same PID file while the current process is alive must fail.
	_, err = process.CreatePIDFile(path)
	if err == nil {
		t.Fatal("expected error for duplicate pid file with live process")
	}
}

func TestPIDFileRemoveIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.pid")
	pf, err := process.CreatePIDFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := pf.Remove(); err != nil {
		t.Fatal(err)
	}
	if err := pf.Remove(); err != nil {
		t.Fatalf("second Remove: %v", err)
	}
}

// stalePID returns the PID of a process that has already exited.
func stalePID(t *testing.T) int {
	t.Helper()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "exit", "0")
	} else {
		cmd = exec.Command("sh", "-c", "exit 0")
	}
	if err := cmd.Run(); err != nil {
		t.Skipf("cannot spawn subprocess for stale PID test: %v", err)
	}
	return cmd.Process.Pid
}

func TestPIDFileStaleDetection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stale.pid")

	pid := stalePID(t)
	if err := os.WriteFile(path, []byte(strconv.Itoa(pid)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	pf, err := process.CreatePIDFile(path)
	if err != nil {
		t.Fatalf("CreatePIDFile should succeed on stale pid file: %v", err)
	}
	defer pf.Remove()
}

func TestPIDFileCorrupted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.pid")

	if err := os.WriteFile(path, []byte("not-a-pid\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Corrupted content is treated as stale and replaced.
	pf, err := process.CreatePIDFile(path)
	if err != nil {
		t.Fatalf("CreatePIDFile on corrupted pid file: %v", err)
	}
	defer pf.Remove()
}
