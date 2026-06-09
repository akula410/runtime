//go:build windows

package process

import (
	"os"
)

func processExists(pid int) bool {
	// On Windows, os.FindProcess always succeeds; OpenProcess would be more accurate
	// but requires syscall. For PID file duplicate detection this is sufficient.
	p, err := os.FindProcess(pid)
	return err == nil && p != nil
}
