package process

// ProcessExists reports whether a process with the given PID is running.
// The implementation is platform-specific.
func ProcessExists(pid int) bool {
	return processExists(pid)
}
