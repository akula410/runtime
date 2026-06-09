package runtime

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// ExternalProcessService runs an external command as a managed Service.
//
// On Unix-like systems the child process is started in its own process group
// so that stopping sends signals to the entire group (including grandchildren).
// On Windows, only the direct child process receives the signal; see the
// platform notes in the README for limitations.
type ExternalProcessService struct {
	name        string
	prog        string
	args        []string
	dir         string
	env         []string
	stdout      io.Writer
	stderr      io.Writer
	killTimeout time.Duration

	mu  sync.Mutex
	cmd *exec.Cmd
}

// NewExternalProcessService creates a service that manages the given program.
func NewExternalProcessService(name, prog string, args ...string) *ExternalProcessService {
	return &ExternalProcessService{
		name:        name,
		prog:        prog,
		args:        args,
		stdout:      os.Stdout,
		stderr:      os.Stderr,
		killTimeout: 5 * time.Second,
	}
}

// WithDir sets the working directory for the process.
func (s *ExternalProcessService) WithDir(dir string) *ExternalProcessService {
	s.dir = dir
	return s
}

// WithEnv sets the environment variables. If nil, the current process env is inherited.
func (s *ExternalProcessService) WithEnv(env []string) *ExternalProcessService {
	s.env = env
	return s
}

// WithStdout redirects the process stdout.
func (s *ExternalProcessService) WithStdout(w io.Writer) *ExternalProcessService {
	s.stdout = w
	return s
}

// WithStderr redirects the process stderr.
func (s *ExternalProcessService) WithStderr(w io.Writer) *ExternalProcessService {
	s.stderr = w
	return s
}

// WithKillTimeout sets how long to wait for the process to exit after sending
// the interrupt signal before sending a hard kill. Default is 5 seconds.
func (s *ExternalProcessService) WithKillTimeout(d time.Duration) *ExternalProcessService {
	s.killTimeout = d
	return s
}

// Name implements Service.
func (s *ExternalProcessService) Name() string { return s.name }

// Start launches the external process and blocks until it exits or ctx is cancelled.
// On cancellation the process group receives an interrupt signal first; if it
// does not exit within KillTimeout, a kill signal is sent.
// Implements Service.
func (s *ExternalProcessService) Start(ctx context.Context) error {
	cmd := exec.Command(s.prog, s.args...)
	if s.dir != "" {
		cmd.Dir = s.dir
	}
	if s.env != nil {
		cmd.Env = s.env
	}
	cmd.Stdout = s.stdout
	cmd.Stderr = s.stderr

	// Platform-specific: put the process in its own group on Unix.
	extProcSetup(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("external process %q: start: %w", s.name, err)
	}

	s.mu.Lock()
	s.cmd = cmd
	s.mu.Unlock()

	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	select {
	case err := <-waitCh:
		s.mu.Lock()
		s.cmd = nil
		s.mu.Unlock()
		return err
	case <-ctx.Done():
		s.interruptAndWait(cmd, waitCh)
		s.mu.Lock()
		s.cmd = nil
		s.mu.Unlock()
		return nil
	}
}

// interruptAndWait sends an interrupt to the process group and waits;
// after killTimeout it sends a kill signal.
func (s *ExternalProcessService) interruptAndWait(cmd *exec.Cmd, waitCh <-chan error) {
	extProcInterrupt(cmd)
	select {
	case <-waitCh:
	case <-time.After(s.killTimeout):
		extProcKill(cmd)
		<-waitCh
	}
}

// Stop sends an interrupt to the process group, if the process is running.
// Implements Service.
func (s *ExternalProcessService) Stop(_ context.Context) error {
	s.mu.Lock()
	cmd := s.cmd
	s.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return extProcStop(cmd)
}

// Health reports whether the process is currently running.
// Implements Service.
func (s *ExternalProcessService) Health(_ context.Context) HealthStatus {
	s.mu.Lock()
	running := s.cmd != nil
	s.mu.Unlock()
	if running {
		return HealthStatus{Name: s.name, Message: "process running"}
	}
	return HealthStatus{Name: s.name, Message: "process not running"}
}
