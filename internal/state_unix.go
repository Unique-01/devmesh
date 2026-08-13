//go:build !windows
// +build !windows

package internal

import (
	"os"
	"syscall"
)

func checkProcessAlive(pid int, p *os.Process) bool {
	if p == nil {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil
}
