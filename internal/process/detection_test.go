package process

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// TestRunWithCallback_DetectsAppOwnPort replicates the Vite scenario: the app
// is started WITHOUT PORT injected, prints its own URL (port 5173 style) and
// binds that port. The manager must report exactly that port.
func TestRunWithCallback_DetectsAppOwnPort(t *testing.T) {
	port := freePort(t)
	cmdStr := `echo "  ➜  Local:   http://localhost:` + strconv.Itoa(port) + `/" && exec python3 -m http.server ` + strconv.Itoa(port) + ` --bind 127.0.0.1`

	mgr := NewManager(cmdStr, 0)
	var out bytes.Buffer

	detected := make(chan int, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	runDone := make(chan error, 1)
	go func() {
		runDone <- mgr.RunWithCallback(ctx, nil, &out, &out, nil, func(p int) {
			select {
			case detected <- p:
			default:
			}
		})
	}()

	select {
	case p := <-detected:
		if p != port {
			t.Errorf("expected detected port %d, got %d", port, p)
		}
	case err := <-runDone:
		t.Fatalf("process finished before detection: %v\noutput: %s", err, out.String())
	case <-time.After(10 * time.Second):
		t.Fatalf("port not detected in time\noutput: %s", out.String())
	}

	cancel()
	<-runDone
}

// TestRunWithCallback_DetectsSilentServerViaSocketPoll covers apps that never
// print a URL: the process-group socket poll must find the bound port.
func TestRunWithCallback_DetectsSilentServerViaSocketPoll(t *testing.T) {
	if _, err := exec.LookPath("lsof"); err != nil {
		t.Skip("lsof not available")
	}
	port := freePort(t)
	// Server binds the port but prints nothing to stdout.
	cmdStr := `exec python3 -c "import socket,time; s=socket.socket(); s.bind(('127.0.0.1',` + strconv.Itoa(port) + `)); s.listen(1); time.sleep(30)" 2>/dev/null`

	mgr := NewManager(cmdStr, 0)
	var out bytes.Buffer

	detected := make(chan int, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	runDone := make(chan error, 1)
	go func() {
		runDone <- mgr.RunWithCallback(ctx, nil, &out, &out, nil, func(p int) {
			select {
			case detected <- p:
			default:
			}
		})
	}()

	select {
	case p := <-detected:
		if p != port {
			t.Errorf("expected detected port %d, got %d", port, p)
		}
	case err := <-runDone:
		t.Fatalf("process finished before detection: %v\noutput: %s", err, out.String())
	case <-time.After(10 * time.Second):
		t.Fatalf("port not detected via socket poll in time\noutput: %s", out.String())
	}

	cancel()
	<-runDone
}

// TestRunWithCallback_ErrAddrInUse replicates the busy-port case: the app's
// intended port is taken by another program, the app crashes, and the manager
// must surface ErrAddrInUse so the caller can restart with another port.
func TestRunWithCallback_ErrAddrInUse(t *testing.T) {
	// Occupy the port from the test process itself (simulates "another program")
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to occupy port: %v", err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port

	cmdStr := `exec python3 -m http.server ` + strconv.Itoa(port) + ` --bind 127.0.0.1`
	mgr := NewManager(cmdStr, 0)
	var out bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	runErr := mgr.RunWithCallback(ctx, nil, &out, &out, nil, nil)
	if !errors.Is(runErr, ErrAddrInUse) {
		t.Fatalf("expected ErrAddrInUse, got %v\noutput: %s", runErr, out.String())
	}
}

// TestRunWithCallback_InjectsPortWhenRequested keeps a regression guard on the
// explicit-port path: PORT must be injected and available in the child env.
func TestRunWithCallback_InjectsPortWhenRequested(t *testing.T) {
	mgr := NewManager("echo PORT=$PORT", 43127)
	var out bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := mgr.Run(ctx, nil, &out, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "PORT=43127") {
		t.Errorf("expected PORT=43127 in output, got %q", out.String())
	}
}
