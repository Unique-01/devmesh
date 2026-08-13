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

	// Set process group for clean termination
	if runtime.GOOS != "windows" {
		m.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		// If running as root, attempt to drop privileges to the user who invoked sudo
		if os.Getuid() == 0 {
			if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
				// We don't have an easy way to switch uid/gid here without affecting the parent
				// but we can at least try to run as the user if we had a more complex setup.
				// For this hackathon, we skip privilege dropping to avoid complexity.
			}
		}
	}

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
		// Also inspect common user binary directories (such as ~/.local/share/pnpm, ~/.nvm, etc.)
		// If SUDO_USER is set, retrieve home dir of SUDO_USER.
		var targetHome string
		if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
			targetHome = fmt.Sprintf("/home/%s", sudoUser)
			if sudoUser == "root" {
				targetHome = "/root"
			}
		} else {
			if home, err := os.UserHomeDir(); err == nil && home != "" {
				targetHome = home
			}
		}

		if targetHome != "" {
			extraPaths = append(extraPaths,
				filepath.Join(targetHome, ".local/bin"),
				filepath.Join(targetHome, ".local/share/pnpm"),
				filepath.Join(targetHome, ".pnpm"),
				filepath.Join(targetHome, ".npm-global/bin"),
			)
			// Check for nvm node versions directory
			nvmDir := filepath.Join(targetHome, ".nvm/versions/node")
			if entries, err := os.ReadDir(nvmDir); err == nil {
				for _, entry := range entries {
					if entry.IsDir() {
						extraPaths = append(extraPaths, filepath.Join(nvmDir, entry.Name(), "bin"))
					}
				}
			}
		}

		// Also check current process or system paths for any additional bin paths (like /home/unic/.local/share/pnpm/bin, etc.)
		if origPath := os.Getenv("PATH"); origPath != "" {
			for _, p := range strings.Split(origPath, ":") {
				if p != "" {
					extraPaths = append(extraPaths, p)
				}
			}
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
		// Kill the entire process group
		_ = syscall.Kill(-pid, syscall.SIGTERM)
	}
}
