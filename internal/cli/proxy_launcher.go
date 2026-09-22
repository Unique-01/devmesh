package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	"devmesh/internal"
)

const (
	proxyAddr80   = "127.0.0.1:80"
	proxyAddr8080 = "127.0.0.1:8080"
)

func findProxyDaemon() (string, error) {
	if internal.PingDaemon("http://" + proxyAddr80) {
		return proxyAddr80, nil
	}

	if internal.PingDaemon("http://" + proxyAddr8080) {
		return proxyAddr8080, nil
	}

	return "", fmt.Errorf("devmesh proxy daemon is not running")
}

// ensureProxyDaemon returns the address of a running proxy daemon.
//
// Binding :80 is the installed systemd service's job (sudo devmesh service
// install); the CLI itself never elevates. If no daemon is reachable, it
// spawns an unprivileged one on :8080 as the current user.
func ensureProxyDaemon() (string, error) {
	if addr, err := findProxyDaemon(); err == nil {
		return addr, nil
	}

	// Acquire lock to avoid concurrent spawn races
	lockPath, err := internal.NewDaemonLock()
	if err != nil {
		return "", err
	}

	if err := internal.AcquireLock(lockPath); err != nil {
		if internal.WaitForDaemon("http://"+proxyAddr8080) == nil {
			return proxyAddr8080, nil
		}
		if internal.WaitForDaemon("http://"+proxyAddr80) == nil {
			return proxyAddr80, nil
		}
		return "", fmt.Errorf("could not acquire lock or connect to daemon: %w", err)
	}
	defer internal.ReleaseLock(lockPath)

	// Check if a daemon came up while we were acquiring the lock
	if addr, err := findProxyDaemon(); err == nil {
		return addr, nil
	}
	
	cmd := exec.Command(os.Args[0], "proxy", "--addr", proxyAddr8080)
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start proxy on %s: %w", proxyAddr8080, err)
	}

	if err := internal.WaitForDaemon("http://" + proxyAddr8080); err != nil {
		return "", fmt.Errorf("failed to start proxy on %s: %w", proxyAddr8080, err)
	}

	fmt.Printf("Note: no DevMesh proxy service found on :80. Spawned an unprivileged daemon on %s.\n", proxyAddr8080)
	fmt.Printf("Tip: run 'sudo devmesh install' once to get an always-on proxy with clean port-80 URLs.\n")
	return proxyAddr8080, nil
}
