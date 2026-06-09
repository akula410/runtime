//go:build !windows

package shutdown

import (
	"os"
	"syscall"
)

var shutdownSignals = []os.Signal{
	syscall.SIGINT,
	syscall.SIGTERM,
}
