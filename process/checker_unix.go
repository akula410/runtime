//go:build !windows

package process

import (
	"os"
	"syscall"
)

func processExists(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 checks existence without sending a real signal.
	err = p.Signal(syscall.Signal(0))
	return err == nil
}
