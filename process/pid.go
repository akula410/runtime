// Package process provides PID file and process lock utilities.
package process

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// PIDFile represents an on-disk PID file.
type PIDFile struct {
	path string
}

// CreatePIDFile writes the current process PID to path and returns a PIDFile handle.
//
// If the file already exists the function reads the stored PID and checks whether
// that process is still alive. A stale PID file (process gone) is removed and a
// fresh one is written. If the stored process is still running, an error is returned.
func CreatePIDFile(path string) (*PIDFile, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if !os.IsExist(err) {
			return nil, fmt.Errorf("process: create pid file %q: %w", path, err)
		}
		// File exists — try to clear it if it is stale.
		if cleanErr := removeIfStale(path); cleanErr != nil {
			return nil, cleanErr
		}
		// Retry once after stale removal.
		f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("process: pid file %q already exists (another instance is running)", path)
		}
	}
	defer f.Close()
	if _, err := fmt.Fprintln(f, strconv.Itoa(os.Getpid())); err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("process: write pid file %q: %w", path, err)
	}
	return &PIDFile{path: path}, nil
}

// removeIfStale removes path when the PID it contains no longer refers to a
// running process. Returns an error when the process is still alive.
func removeIfStale(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // already removed — fine
		}
		return fmt.Errorf("process: read pid file %q: %w", path, err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		// Corrupted file — remove it.
		return os.Remove(path)
	}
	if ProcessExists(pid) {
		return fmt.Errorf("process: pid file %q contains PID %d which is still running", path, pid)
	}
	return os.Remove(path)
}

// Remove deletes the PID file. Safe to call multiple times.
func (p *PIDFile) Remove() error {
	if err := os.Remove(p.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("process: remove pid file %q: %w", p.path, err)
	}
	return nil
}

// Path returns the path of the PID file.
func (p *PIDFile) Path() string { return p.path }
