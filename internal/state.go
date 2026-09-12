package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProjectState represents the running or registered state of a DevMesh project.
type ProjectState struct {
	Name      string `json:"name" yaml:"name"`
	Domain    string `json:"domain" yaml:"domain"`
	Port      int    `json:"port" yaml:"port"`
	PID       int    `json:"pid" yaml:"pid"`
	Cmd       string `json:"cmd" yaml:"cmd"`
	Directory string `json:"directory" yaml:"directory"`
}

// GetDevMeshDir returns the DevMesh state directory (~/.devmesh), resolved
// from the user's home directory so it is portable across Linux, macOS, and
// Windows and never requires elevated privileges.
func GetDevMeshDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}
	dir := filepath.Join(home, ".devmesh")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create devmesh dir: %w", err)
	}
	return dir, nil
}

// GetStateFilePath returns the path to state/projects.json or state for a given project name.
func GetStateFilePath(projectName string) (string, error) {
	dir, err := GetDevMeshDir()
	if err != nil {
		return "", err
	}
	statesDir := filepath.Join(dir, "states")
	if err := os.MkdirAll(statesDir, 0755); err != nil {
		return "", err
	}
	sanitized := sanitizeProjectName(projectName)
	return filepath.Join(statesDir, sanitized+".json"), nil
}

// SaveProjectState saves or updates project state.
func SaveProjectState(state ProjectState) error {
	path, err := GetStateFilePath(state.Name)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal project state: %w", err)
	}
	// Write with 0644 to ensure readability for the user, even if created as root
	return os.WriteFile(path, data, 0644)
}

// RemoveProjectState deletes the project state file.
func RemoveProjectState(projectName string) error {
	path, err := GetStateFilePath(projectName)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove project state: %w", err)
	}
	return nil
}

// LoadProjectState loads project state by name.
func LoadProjectState(projectName string) (ProjectState, error) {
	path, err := GetStateFilePath(projectName)
	if err != nil {
		return ProjectState{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ProjectState{}, err
	}
	var state ProjectState
	if err := json.Unmarshal(data, &state); err != nil {
		return ProjectState{}, fmt.Errorf("failed to unmarshal project state: %w", err)
	}
	return state, nil
}

// ListAllProjectStates returns all saved project states across the system.
func ListAllProjectStates() ([]ProjectState, error) {
	dir, err := GetDevMeshDir()
	if err != nil {
		return nil, err
	}
	statesDir := filepath.Join(dir, "states")
	if _, err := os.Stat(statesDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(statesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read states dir: %w", err)
	}

	var states []ProjectState
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(statesDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var state ProjectState
		if err := json.Unmarshal(data, &state); err == nil {
			states = append(states, state)
		}
	}
	return states, nil
}

// IsProcessRunning checks if a process with the given PID is currently running.
func IsProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return checkProcessAlive(pid, process)
}
