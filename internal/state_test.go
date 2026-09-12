package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetDevMeshDir_HomeBased(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	dir, err := GetDevMeshDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(fakeHome, ".devmesh")
	if dir != expected {
		t.Errorf("expected %s, got %s", expected, dir)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Errorf("expected %s to exist as a directory", dir)
	}
}

func TestProjectStateRoundtrip_HomeBased(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	state := ProjectState{
		Name:      "vault",
		Domain:    "vault.localhost",
		Port:      5173,
		PID:       0,
		Cmd:       "pnpm dev",
		Directory: fakeHome,
	}
	if err := SaveProjectState(state); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	loaded, err := LoadProjectState("vault")
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if loaded.Name != "vault" || loaded.Port != 5173 || loaded.Domain != "vault.localhost" {
		t.Errorf("state roundtrip mismatch: %+v", loaded)
	}

	states, err := ListAllProjectStates()
	if err != nil {
		t.Fatalf("failed to list states: %v", err)
	}
	if len(states) != 1 || states[0].Name != "vault" {
		t.Errorf("expected [vault], got %+v", states)
	}

	if err := RemoveProjectState("vault"); err != nil {
		t.Fatalf("failed to remove state: %v", err)
	}
	if _, err := LoadProjectState("vault"); err == nil {
		t.Error("expected error loading removed state")
	}
}
