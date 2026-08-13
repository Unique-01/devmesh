package process

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
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
	return m.RunWithCallback(ctx, stdin, stdout, stderr, nil)
}

// RunWithCallback runs the process and invokes an optional callback function once started with PID.
func (m *Manager) RunWithCallback(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, onStart func(pid int)) error {
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

	// Inject PORT and preserve/enhance environment
	env := os.Environ()
	portStr := strconv.Itoa(m.port)
	env = append(env, fmt.Sprintf("PORT=%s", portStr))

	// If running under sudo, ensure PATH includes common user binary locations or SUDO_USER environment
	hasPath := false
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			hasPath = true
			break
		}
	}
	if !hasPath || os.Getuid() == 0 {
		// When running as root (e.g. sudo), PATH might be restricted (/usr/sbin:/usr/bin:/sbin:/bin).
		// Let's ensure common user paths / homebrew / pnpm / nvm / npm paths are included if missing.
		extraPaths := []string{
			"/usr/local/bin",
			"/usr/bin",
			"/bin",
			"/usr/sbin",
			"/sbin",
		}
		// If SUDO_USER is set, we can check or guess home directory bin paths
		if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
			extraPaths = append(extraPaths,
				fmt.Sprintf("/home/%s/.local/bin", sudoUser),
				fmt.Sprintf("/home/%s/.pnpm", sudoUser),
				fmt.Sprintf("/home/%s/.npm-global/bin", sudoUser),
			)
		}
		// Also add common npm/pnpm global paths
		homeDir, err := os.UserHomeDir()
		if err == nil && homeDir != "" {
			extraPaths = append(extraPaths,
				filepath.Join(homeDir, ".local/bin"),
				filepath.Join(homeDir, ".pnpm"),
				filepath.Join(homeDir, ".npm-global/bin"),
			)
		}
		// Prepend or append to PATH
		existingPath := ""
		for i, e := range env {
			if strings.HasPrefix(e, "PATH=") {
				existingPath = strings.TrimPrefix(e, "PATH=")
				env[i] = fmt.Sprintf("PATH=%s:%s", strings.Join(extraPaths, ":"), existingPath)
				hasPath = true
				break
			}
		}
		if !hasPath {
			env = append(env, fmt.Sprintf("PATH=%s", strings.Join(extraPaths, ":")))
		}
	}

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

	if onStart != nil {
		onStart(m.PID())
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
