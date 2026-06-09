//go:build !windows

package runtime

import (
	"os/exec"
	"syscall"
)

// extProcSetup puts the child process in its own process group.
// This ensures that interrupt/kill signals reach the whole process subtree,
// not just the direct child (e.g. sh spawning sleep).
func extProcSetup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// extProcInterrupt sends SIGINT to the process group (negative PID = pgid).
func extProcInterrupt(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
}

// extProcKill sends SIGKILL to the process group.
func extProcKill(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}

// extProcStop sends SIGINT to the process group and returns any error.
func extProcStop(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
}
