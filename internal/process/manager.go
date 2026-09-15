package process

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
)

// ErrAddrInUse is returned when the child process failed to start because the
// port it intended to bind is already in use by another program.
var ErrAddrInUse = errors.New("intended port is already in use")

// Manager handles spawning, tracking, and cleaning up child processes.
type Manager struct {
	cmdStr string
	port   int
	cmd    *exec.Cmd
}

// NewManager creates a new process manager for the given command string and port.
// A port of 0 means "do not inject PORT": the child runs on whatever port it
// intends to use, and the actual port can be discovered via onPortDetected.
func NewManager(cmdStr string, port int) *Manager {
	return &Manager{
		cmdStr: cmdStr,
		port:   port,
	}
}

// Port returns the assigned PORT (0 when the child chooses its own port).
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

// Run spawns the command with PORT injected if specified, forwards stdio,
// tracks PID, and handles graceful termination on signals (Ctrl+C / SIGINT / SIGTERM).
func (m *Manager) Run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
	return m.RunWithCallback(ctx, stdin, stdout, stderr, nil, nil)
}

// RunWithCallback runs the process, invokes an optional callback once started
// with the PID, and an optional callback when the port the process actually
// bound is discovered (via output parsing or socket inspection).
//
// If the child fails to bind its intended port (address already in use), the
// returned error wraps ErrAddrInUse so callers can retry with another port.
func (m *Manager) RunWithCallback(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, onStart func(pid int), onPortDetected func(port int)) error {
	var shell, flag string
	if runtime.GOOS == "windows" {
		shell = "cmd"
		flag = "/c"
	} else {
		shell = "sh"
		flag = "-c"
	}

	m.cmd = exec.CommandContext(ctx, shell, flag, m.cmdStr)

	if runtime.GOOS != "windows" {
		m.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	// Inject PORT only when explicitly requested. With port == 0 the child
	// keeps its own default port.
	env := os.Environ()
	if m.port > 0 {
		env = append(env, fmt.Sprintf("PORT=%d", m.port))
	}

	m.cmd.Env = env

	outWriter := stdout
	if outWriter == nil {
		outWriter = os.Stdout
	}
	errWriter := stderr
	if errWriter == nil {
		errWriter = os.Stderr
	}

	// Tee child output to the terminal and through scanners that watch for the
	// bound port and for bind failures (address already in use).
	prOut, pwOut := io.Pipe()
	prErr, pwErr := io.Pipe()
	m.cmd.Stdout = io.MultiWriter(outWriter, pwOut)
	m.cmd.Stderr = io.MultiWriter(errWriter, pwErr)

	portChan := make(chan int, 8)
	addrInUseChan := make(chan struct{}, 1)
	go scanStreamForPort(prOut, portChan, addrInUseChan)
	go scanStreamForPort(prErr, portChan, addrInUseChan)

	// Pipes must stay open until exec's internal copier goroutines finish
	// (i.e. Wait returns); every exit path below drains errChan first.
	defer pwOut.Close()
	defer pwErr.Close()

	if stdin != nil {
		m.cmd.Stdin = stdin
	} else {
		m.cmd.Stdin = os.Stdin
	}

	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	pid := m.PID()
	if onStart != nil {
		onStart(pid)
	}

	portDetectCtx,cancelPortDetect := context.WithCancel(ctx)
	defer cancelPortDetect()
	detectPort(portDetectCtx,pid, portChan, onPortDetected)

	exitErr := m.handleExit(ctx, addrInUseChan)

	return exitErr
}

func (m *Manager) handleExit(ctx context.Context, addrInUseChan <-chan struct{}) error {
	exitSigChan := make(chan os.Signal, 1)
	signal.Notify(exitSigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(exitSigChan)

	errChan := make(chan error, 1)
	go func() {
		errChan <- m.cmd.Wait()
	}()

	var exitErr error
	select {
	case sig := <-exitSigChan:
		m.terminateProcess()
		<-errChan
		exitErr = fmt.Errorf("process terminated by signal %v", sig)

	case <-ctx.Done():
		m.terminateProcess()
		<-errChan
		exitErr = ctx.Err()

	case <-addrInUseChan:
		m.terminateProcess()
		processExitErr := <-errChan
		if processExitErr == nil {
			exitErr = ErrAddrInUse
		} else {
			exitErr = fmt.Errorf("%w: %v", ErrAddrInUse, processExitErr)
		}

	case exitErr = <-errChan:
	}

	return exitErr
}

// terminateProcess cleanly terminates the process and its children if possible.
func (m *Manager) terminateProcess() {
	if m.cmd == nil || m.cmd.Process == nil {
		return
	}

	pid := m.cmd.Process.Pid
	if runtime.GOOS == "windows" {
		_ = m.cmd.Process.Kill()
	} else {
		_ = syscall.Kill(-pid, syscall.SIGTERM)
	}
}
