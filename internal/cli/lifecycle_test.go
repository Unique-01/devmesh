package cli

import (
	"os"
	"testing"

	"devmesh/internal"
)

func TestLifecycleCommands(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate state writes to a temp dir
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer os.Chdir(origDir)

	// Create .devmesh.yaml
	configContent := "name: vault\ndomain: vault.localhost\ncmd: \"echo hello\"\nport: 43127\n"
	if err := os.WriteFile(".devmesh.yaml", []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Save project state
	state := internal.ProjectState{
		Name:      "vault",
		Domain:    "vault.localhost",
		Port:      43127,
		PID:       0,
		Cmd:       "echo hello",
		Directory: tmpDir,
	}
	if err := internal.SaveProjectState(state); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Test status command
	if err := statusCmd.RunE(statusCmd, []string{}); err != nil {
		t.Errorf("statusCmd failed: %v", err)
	}

	// Test stop command
	if err := stopCmd.RunE(stopCmd, []string{}); err != nil {
		t.Errorf("stopCmd failed: %v", err)
	}

	// Verify state after stop (PID should be 0)
	loaded, err := internal.LoadProjectState("vault")
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if loaded.PID != 0 {
		t.Errorf("expected PID 0 after down, got %d", loaded.PID)
	}
}
