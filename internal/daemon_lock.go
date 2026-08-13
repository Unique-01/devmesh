package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// DaemonLock manages the exclusive lock for the proxy daemon.
type DaemonLock struct {
	lockFile string
}

func NewDaemonLock() (string, error) {
	dir, err := GetDevMeshDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "daemon.lock"), nil
}

// AcquireLock attempts to acquire the daemon lock, handling staleness.
func AcquireLock(lockPath string) error {
	for {
		pidData, err := os.ReadFile(lockPath)
		if err == nil {
			pid, err := strconv.Atoi(string(pidData))
			if err == nil && isAlive(pid) {
				return fmt.Errorf("daemon already running with PID %d", pid)
			}
			// Stale lock, remove it
			os.Remove(lockPath)
		}

		// Try to create file exclusively
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			defer f.Close()
			_, err = f.WriteString(strconv.Itoa(os.Getpid()))
			return err
		}

		// If someone else grabbed it between check and creation, wait and retry
		time.Sleep(100 * time.Millisecond)
	}
}

func ReleaseLock(lockPath string) {
	os.Remove(lockPath)
}

func isAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, sending signal 0 checks for process existence
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
