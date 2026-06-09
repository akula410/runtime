package process_test

import (
	"os"
	"path/filepath"
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

	_, err = process.CreatePIDFile(path)
	if err == nil {
		t.Fatal("expected error for duplicate pid file")
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
	// second remove should not error
	if err := pf.Remove(); err != nil {
		t.Fatalf("second Remove: %v", err)
	}
}
