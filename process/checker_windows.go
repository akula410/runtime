//go:build windows

package process

import "syscall"

// processQueryLimitedInformation allows querying process information without
// requiring higher privileges (available since Windows Vista).
const processQueryLimitedInformation = 0x1000

// processExists checks whether a process with the given PID is alive on Windows.
// It uses OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION) which properly fails for
// non-existent PIDs, unlike os.FindProcess which always succeeds.
func processExists(pid int) bool {
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return false
	}
	_ = syscall.CloseHandle(h)
	return true
}
