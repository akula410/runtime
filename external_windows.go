//go:build windows

package runtime

import (
	"os"
	"os/exec"
)

// extProcSetup is a no-op on Windows.
//
// TODO: implement Windows Job Objects for full child-process tracking.
// Without Job Objects, child processes spawned by the managed process
// (e.g. cmd.exe spawning another program) are not automatically terminated.
func extProcSetup(_ *exec.Cmd) {}

// extProcInterrupt sends os.Interrupt (CTRL_C_EVENT) to the process.
// Note: this only works reliably for console applications in the same console group.
func extProcInterrupt(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Signal(os.Interrupt)
	}
}

// extProcKill terminates the process immediately.
func extProcKill(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

// extProcStop sends os.Interrupt to the process.
func extProcStop(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Signal(os.Interrupt)
}
