// Package process provides PID file and process lock utilities.
package process

import (
	"fmt"
	"os"
	"strconv"
)

// PIDFile represents an on-disk PID file.
type PIDFile struct {
	path string
}

// CreatePIDFile writes the current process PID to path and returns a PIDFile handle.
// Returns an error if the file already exists (possible duplicate process).
func CreatePIDFile(path string) (*PIDFile, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("process: pid file %q already exists (another instance may be running)", path)
		}
		return nil, fmt.Errorf("process: create pid file %q: %w", path, err)
	}
	defer f.Close()
	if _, err := fmt.Fprintln(f, strconv.Itoa(os.Getpid())); err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("process: write pid file %q: %w", path, err)
	}
	return &PIDFile{path: path}, nil
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
