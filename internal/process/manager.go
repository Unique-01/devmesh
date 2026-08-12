package process

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
)

// Manager handles spawning, tracking, and cleaning up child processes.
type Manager struct {
	cmdStr string
	port   int
	cmd    *exec.Cmd
}

// NewManager creates a new process manager for the given command string and port.
func NewManager(cmdStr string, port int) *Manager {
	return &Manager{
		cmdStr: cmdStr,
		port:   port,
	}
}

// Port returns the assigned PORT.
func (m *Manager) Port() int {
	return m.port
}

// PID returns the process ID of the running command, or 0 if not running.
func (m *Manager) PID() int {
	if m.cmd != nil && m.cmd.Process != nil {
		return m.cmd.Process.Pid
	}
	return 0
}

// Run spawns the command with PORT injected into the environment, forwards stdio,
// tracks PID, and handles graceful termination on signals (Ctrl+C / SIGINT / SIGTERM).
func (m *Manager) Run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
	// Determine shell execution based on OS
	var shell, flag string
	if runtime.GOOS == "windows" {
		shell = "cmd"
		flag = "/c"
	} else {
		shell = "sh"
		flag = "-c"
	}

	m.cmd = exec.CommandContext(ctx, shell, flag, m.cmdStr)

	// Inject PORT and preserve existing environment
	env := os.Environ()
	portStr := strconv.Itoa(m.port)
	env = append(env, fmt.Sprintf("PORT=%s", portStr))
	m.cmd.Env = env

	// Forward stdio
	if stdin != nil {
		m.cmd.Stdin = stdin
	} else {
		m.cmd.Stdin = os.Stdin
	}

	if stdout != nil {
		m.cmd.Stdout = stdout
	} else {
		m.cmd.Stdout = os.Stdout
	}

	if stderr != nil {
		m.cmd.Stderr = stderr
	} else {
		m.cmd.Stderr = os.Stderr
	}

	// Set up signal handling for clean termination (Ctrl+C)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Channel to capture command completion error
	errChan := make(chan error, 1)

	// Start the process
	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Log PID / info if needed or return PID via callback / struct
	// Go routine to wait for command exit
	go func() {
		errChan <- m.cmd.Wait()
	}()

	// Wait for either signal, context cancellation, or command completion
	select {
	case sig := <-sigChan:
		// Received Ctrl+C / SIGINT / SIGTERM
		m.terminateProcess(sig)
		// Wait for process to exit after termination signal
		<-errChan
		return fmt.Errorf("process terminated by signal %v", sig)
	case <-ctx.Done():
		m.terminateProcess(syscall.SIGTERM)
		<-errChan
		return ctx.Err()
	case err := <-errChan:
		return err
	}
}

// terminateProcess cleanly terminates the process and its children if possible.
func (m *Manager) terminateProcess(sig os.Signal) {
	if m.cmd == nil || m.cmd.Process == nil {
		return
	}

	pid := m.cmd.Process.Pid
	if runtime.GOOS == "windows" {
		_ = m.cmd.Process.Kill()
	} else {
		// Send signal to process group if process group was created, or process itself.
		// Sending negative PID sends signal to process group.
		pgid, err := syscall.Getpgid(pid)
		if err == nil && pgid > 0 {
			_ = syscall.Kill(-pgid, syscall.SIGTERM)
		} else {
			_ = m.cmd.Process.Signal(sig)
		}
	}
}
