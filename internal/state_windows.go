//go:build windows
// +build windows

package internal

import (
	"os"

	"golang.org/x/sys/windows"
)

func checkProcessAlive(pid int, p *os.Process) bool {
	if p == nil {
		return false
	}
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)

	event, err := windows.WaitForSingleObject(h, 0)
	if err != nil {
		return false
	}
	return event == uint32(windows.WAIT_TIMEOUT) // timeout = still running
}