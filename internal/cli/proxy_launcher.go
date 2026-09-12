package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	"devmesh/internal"
)

// ensureProxyDaemon returns the address of a running proxy daemon.
//
// Binding :80 is the installed systemd service's job (sudo devmesh service
// install); the CLI itself never elevates. If no daemon is reachable, it
// spawns an unprivileged one on :8080 as the current user.
func ensureProxyDaemon() (string, error) {
	addr := "127.0.0.1:80"
	if internal.PingDaemon("http://" + addr) {
		return addr, nil
	}

	addr8080 := "127.0.0.1:8080"
	if internal.PingDaemon("http://" + addr8080) {
		return addr8080, nil
	}

	// Acquire lock to avoid concurrent spawn races
	lockPath, err := internal.NewDaemonLock()
	if err != nil {
		return "", err
	}

	if err := internal.AcquireLock(lockPath); err != nil {
		if internal.WaitForDaemon("http://"+addr8080) == nil {
			return addr8080, nil
		}
		if internal.WaitForDaemon("http://"+addr) == nil {
			return addr, nil
		}
		return "", fmt.Errorf("could not acquire lock or connect to daemon: %w", err)
	}
	defer internal.ReleaseLock(lockPath)

	// Check if a daemon came up while we were acquiring the lock
	if internal.PingDaemon("http://" + addr) {
		return addr, nil
	}
	if internal.PingDaemon("http://" + addr8080) {
		return addr8080, nil
	}

	cmd := exec.Command(os.Args[0], "proxy", "--addr", addr8080)
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start proxy on %s: %w", addr8080, err)
	}

	if err := internal.WaitForDaemon("http://" + addr8080); err != nil {
		return "", fmt.Errorf("failed to start proxy on %s: %w", addr8080, err)
	}

	fmt.Printf("Note: no DevMesh proxy service found on :80. Spawned an unprivileged daemon on %s.\n", addr8080)
	fmt.Printf("Tip: run 'sudo devmesh install' once to get an always-on proxy with clean port-80 URLs.\n")
	return addr8080, nil
}
