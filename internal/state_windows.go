//go:build windows
// +build windows

package internal

import (
	"os"
)

func checkProcessAlive(pid int, p *os.Process) bool {
	if p == nil {
		return false
	}
	// On Windows, Signal with nil or checking process existence
	err := p.Signal(os.Signal(0))
	return err == nil
}
