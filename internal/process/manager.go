package process

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ErrAddrInUse is returned when the child process failed to start because the
// port it intended to bind is already in use by another program. Callers can
// use errors.Is to decide to restart the command with an alternative port.
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
	// keeps its own default port (e.g. Vite on 5173).
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

	// Discover the port the child actually bound. Primary: parse startup
	// output (URL/port patterns). Fallback: poll listening TCP sockets of the
	// child's whole process group (works for servers that print no URL).
	done := make(chan struct{})
	defer close(done)
	if onPortDetected != nil {
		go func() {
			ticker := time.NewTicker(400 * time.Millisecond)
			defer ticker.Stop()
			timeout := time.After(30 * time.Second)

			for {
				select {
				case <-done:
					return
				case <-timeout:
					return
				case p := <-portChan:
					if p > 0 {
						onPortDetected(p)
						return
					}
				case <-ticker.C:
					if p := detectPortViaSocketPoll(pid); p > 0 {
						onPortDetected(p)
						return
					}
				}
			}
		}()
	}

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
		m.terminateProcess(sig)
		<-errChan
		exitErr = fmt.Errorf("process terminated by signal %v", sig)

	case <-ctx.Done():
		m.terminateProcess(syscall.SIGTERM)
		<-errChan
		exitErr = ctx.Err()

	case <-addrInUseChan:
		m.terminateProcess(syscall.SIGTERM)
		processExitErr := <-errChan
		if processExitErr == nil {
			exitErr = ErrAddrInUse
		} else {
			exitErr = fmt.Errorf("%w: %v", ErrAddrInUse, exitErr)
		}

	case exitErr = <-errChan:
	}

	return exitErr
}

var (
	reAddrInUse = regexp.MustCompile(`(?i)eaddrinuse|address already in use|only one usage of each socket address`)
	// Lines saying a port is busy/taken are NOT the port the server bound on
	// (e.g. Vite's "Port 5173 is in use, trying another one...").
	reBusyPort = regexp.MustCompile(`(?i)\bin use\b|\bbusy\b|trying another|already in use`)
	reURLPort  = regexp.MustCompile(`(?:localhost|127\.0\.0\.1|0\.0\.0\.0|\*|\[::1\]):(\d{2,5})`)
	rePortWord = regexp.MustCompile(`(?i)\bport["']?\s*[:=]\s*["']?(\d{2,5})\b`)
)

// scanStreamForPort scans a child's output stream for the port it bound and
// for bind failures, delivering results on the provided channels.
func scanStreamForPort(r io.Reader, portChan chan<- int, addrInUseChan chan<- struct{}) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()

		if reAddrInUse.MatchString(line) {
			select {
			case addrInUseChan <- struct{}{}:
			default:
			}
			continue
		}
		if reBusyPort.MatchString(line) {
			continue
		}

		port := 0
		if m := reURLPort.FindStringSubmatch(line); len(m) >= 2 {
			port, _ = strconv.Atoi(m[1])
		}
		if port == 0 {
			if m := rePortWord.FindStringSubmatch(line); len(m) >= 2 {
				port, _ = strconv.Atoi(m[1])
			}
		}
		if port >= 1024 && port <= 65535 {
			select {
			case portChan <- port:
			default:
			}
		}
	}
}

// detectPortViaSocketPoll finds a TCP port in LISTEN state owned by any
// process in the given process group. The child runs in its own process group
// (Setpgid), so inspecting the group covers the shell wrapper plus the actual
// server process (e.g. sh -> pnpm -> vite), which a plain `lsof -p <pid>`
// would miss.
func detectPortViaSocketPoll(pgid int) int {
	if pgid <= 0 {
		return 0
	}
	out, err := exec.Command("lsof", "-a", "-iTCP", "-sTCP:LISTEN", "-P", "-n", "-g", strconv.Itoa(pgid)).Output()
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "COMMAND") {
			continue
		}
		line = strings.ReplaceAll(line, "(LISTEN)", "")
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := fields[len(fields)-1]
		idx := strings.LastIndex(name, ":")
		if idx == -1 {
			continue
		}
		if p, err := strconv.Atoi(name[idx+1:]); err == nil && p >= 1024 && p <= 65535 {
			return p
		}
	}
	return 0
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
		_ = syscall.Kill(-pid, syscall.SIGTERM)
	}
}
