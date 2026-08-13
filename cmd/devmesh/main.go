package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"devmesh/internal/cli"
)

func main() {
	if needsSudo() {
		args := append([]string{os.Args[0]}, os.Args[1:]...)
		cmd := exec.Command("sudo", args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to elevate: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	cli.Execute()
}

func needsSudo() bool {
	if os.Geteuid() == 0 || runtime.GOOS == "windows" {
		return false
	}
	// Check if any argument requires root
	for _, arg := range os.Args {
		switch arg {
		case "up", "down", "remove", "proxy":
			return true
		}
	}
	return false
}
