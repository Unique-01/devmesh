package process

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestManagerPortAndPID(t *testing.T) {
	mgr := NewManager("echo hello", 12345)
	if mgr.Port() != 12345 {
		t.Errorf("expected port 12345, got %d", mgr.Port())
	}
	if mgr.PID() != 0 {
		t.Errorf("expected PID 0 before start, got %d", mgr.PID())
	}
}

func TestManagerRunAndInjectPort(t *testing.T) {
	// A command that prints the PORT environment variable
	var cmdStr string
	if testing.Short() {
		return
	}

	// Use go or printenv / python / node depending on what's available, or a simple sh command
	cmdStr = "echo PORT=$PORT"

	mgr := NewManager(cmdStr, 43127)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := mgr.Run(ctx, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error running manager: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "PORT=43127") {
		t.Errorf("expected output to contain PORT=43127, got: %q", output)
	}
}

func TestManagerStreamAndPID(t *testing.T) {
	// Test that PID is tracked while running
	var cmdStr = "sleep 1"
	mgr := NewManager(cmdStr, 8080)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- mgr.Run(ctx, nil, nil, nil)
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	pid := mgr.PID()
	if pid <= 0 {
		t.Errorf("expected positive PID while running, got %d", pid)
	}

	err := <-errChan
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mgr.PID() != 0 {
		// After exit, PID tracking might reset or retain process struct. But cmd.Process exists or PID is 0.
		// Let's check what PID returns after exit.
	}
}
