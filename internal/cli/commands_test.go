package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"devmesh/internal"
)

// spawnDetached starts a long-running process in its own process group (like
// devmesh does for dev servers) and returns it.
func spawnDetached(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("sleep", "30")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to spawn test process: %v", err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_ = cmd.Wait()
	})
	return cmd
}

// reapExited waits until the process has been waited on (avoiding a zombie
// that would still respond to existence checks).
func reapExited(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("process %d did not exit after stop", cmd.Process.Pid)
	}
}

// savedProjectState builds and persists a fake project state for tests.
func savedProjectState(t *testing.T, name string, pid int, dir string) internal.ProjectState {
	t.Helper()
	state := internal.ProjectState{
		Name:      name,
		Domain:    name + ".localhost",
		Port:      0,
		PID:       pid,
		Cmd:       "echo " + name,
		Directory: dir,
	}
	if err := internal.SaveProjectState(state); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}
	return state
}

func TestRunStopNamed_KillsProcessAndClearsState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cmd := spawnDetached(t)
	pid := cmd.Process.Pid

	savedProjectState(t, "sleepy", pid, t.TempDir())

	if err := runStopNamed("sleepy"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	reapExited(t, cmd) // SIGTERM delivered; wait so the PID is truly gone
	if internal.IsProcessRunning(pid) {
		t.Errorf("expected process %d to be terminated", pid)
	}
	loaded, err := internal.LoadProjectState("sleepy")
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if loaded.PID != 0 {
		t.Errorf("expected PID 0 after stop, got %d", loaded.PID)
	}
}

func TestRunStopNamed_UnknownProject(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := runStopNamed("does-not-exist"); err == nil {
		t.Error("expected error for unknown project name")
	}
}

func TestRunStartNamed_AlreadyRunning(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cmd := spawnDetached(t)

	savedProjectState(t, "sleepy", cmd.Process.Pid, t.TempDir())

	err := runStartNamed("sleepy")
	if err == nil {
		t.Fatal("expected 'already running' error")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("expected 'already running' in error, got: %v", err)
	}
}

func TestRunStartNamed_UnknownAndMissingDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := runStartNamed("does-not-exist"); err == nil {
		t.Error("expected error for unknown project name")
	}

	// Saved project whose directory vanished
	state := savedProjectState(t, "ghost", 0, t.TempDir()+"/gone")
	state.PID = 0
	if err := internal.SaveProjectState(state); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}
	if err := runStartNamed("ghost"); err == nil {
		t.Error("expected error for missing project directory")
	}
}

func TestInstallBinary_ReplacesExistingFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src-bin")
	dst := filepath.Join(dir, "dst-bin")

	if err := os.WriteFile(src, []byte("new-binary-content"), 0755); err != nil {
		t.Fatalf("failed to write src: %v", err)
	}
	// Pre-existing "installed" binary (simulates a previous install)
	if err := os.WriteFile(dst, []byte("old-binary-content"), 0755); err != nil {
		t.Fatalf("failed to write dst: %v", err)
	}

	if err := installBinary(src, dst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read dst: %v", err)
	}
	if string(data) != "new-binary-content" {
		t.Errorf("expected dst to be replaced, got %q", string(data))
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("failed to stat dst: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("expected 0755 on dst, got %v", info.Mode().Perm())
	}
	// No temp leftovers
	if _, err := os.Stat(dst + ".tmp"); !os.IsNotExist(err) {
		t.Error("expected temp file to be cleaned up")
	}
}
