package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	"devmesh/internal"
)

func ensureProxyDaemon() (string, error) {
	addr := "127.0.0.1:80"
	
	// Check if already running
	err := internal.WaitForDaemon("http://" + addr)
	if err == nil {
		return addr, nil
	}

	// Try 8080 fallback
	addr8080 := "127.0.0.1:8080"
	err = internal.WaitForDaemon("http://" + addr8080)
	if err == nil {
		return addr8080, nil
	}

	// Acquire lock and spawn
	lockPath, err := internal.NewDaemonLock()
	if err != nil {
		return "", err
	}

	err = internal.AcquireLock(lockPath)
	if err != nil {
		// Wait for existing daemon to start
		if err := internal.WaitForDaemon("http://" + addr); err == nil {
			return addr, nil
		}
		if err := internal.WaitForDaemon("http://" + addr8080); err == nil {
			return addr8080, nil
		}
		return "", fmt.Errorf("could not acquire lock or connect to daemon: %w", err)
	}
	defer internal.ReleaseLock(lockPath)

	// Check if it started while we were acquiring lock
	if err := internal.WaitForDaemon("http://" + addr); err == nil {
		return addr, nil
	}

	// Try starting on 80
	cmd := exec.Command(os.Args[0], "proxy", "--addr", addr)
	
	// Detach
	if runtime.GOOS == "windows" {
		// Windows detachment - rely on standard cmd start for now if needed, or omit SysProcAttr
	} else {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}

	// Try starting
	if err := cmd.Start(); err != nil {
		// Fallback to 8080
		addr = addr8080
		fmt.Printf("Note: Could not bind to port 80 (permission denied). Falling back to %s\n", addr)
		cmd = exec.Command(os.Args[0], "proxy", "--addr", addr)
		if runtime.GOOS != "windows" {
			cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		}
		if err := cmd.Start(); err != nil {
			return "", fmt.Errorf("failed to start proxy on %s: %w", addr, err)
		}
	}

	if err := internal.WaitForDaemon("http://" + addr); err != nil {
		return "", fmt.Errorf("failed to start proxy on %s: %w", addr, err)
	}
	return addr, nil
}
