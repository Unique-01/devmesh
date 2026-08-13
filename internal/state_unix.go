//go:build !windows
// +build !windows

package internal

import (
	"errors"
	"os"
	"syscall"
)

func checkProcessAlive(pid int, p *os.Process) bool {
	if p == nil {
		return false
	}

	err := syscall.Kill(pid, 0)
	if err == nil {
		return true // signalable -> definitely alive
	}

	if errno, ok := errors.AsType[syscall.Errno](err); ok {
		switch errno {
		case syscall.ESRCH:
			return false // no such process -> definitely dead
		case syscall.EPERM:
			return true // exists, owned by someone else (e.g. root) -> alive
		}
	}

	// Unknown error — fail safe rather than reporting a possibly-live
	// root-owned process as stopped.
	return false
}