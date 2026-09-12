package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func setupFakeState(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SUDO_USER", "") // ensure the sudo-invoker branch is not taken
	stateDir := filepath.Join(home, ".devmesh")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("failed to create fake state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "states.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to create fake state file: %v", err)
	}
	return home
}

func TestUninstall_KeepData(t *testing.T) {
	home := setupFakeState(t)

	if err := uninstallCmd.Flags().Set("keep-data", "true"); err != nil {
		t.Fatalf("failed to set flag: %v", err)
	}
	defer uninstallCmd.Flags().Set("keep-data", "false")

	if err := uninstallCmd.RunE(uninstallCmd, []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".devmesh")); err != nil {
		t.Errorf("expected ~/.devmesh to be kept with --keep-data")
	}
}

func TestUninstall_ConfirmYes(t *testing.T) {
	home := setupFakeState(t)

	if err := uninstallCmd.Flags().Set("keep-data", "false"); err != nil {
		t.Fatalf("failed to set flag: %v", err)
	}
	uninstallInput = strings.NewReader("y\n")
	defer func() { uninstallInput = os.Stdin }()

	if err := uninstallCmd.RunE(uninstallCmd, []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".devmesh")); !os.IsNotExist(err) {
		t.Errorf("expected ~/.devmesh to be removed after confirmation")
	}
}

func TestUninstall_ConfirmNo(t *testing.T) {
	home := setupFakeState(t)

	uninstallInput = strings.NewReader("n\n")
	defer func() { uninstallInput = os.Stdin }()

	if err := uninstallCmd.RunE(uninstallCmd, []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".devmesh")); err != nil {
		t.Errorf("expected ~/.devmesh to be kept after declining")
	}
}

func TestServiceHomeDir_UsesHomeEnv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SUDO_USER", "")

	got, err := serviceHomeDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != home {
		t.Errorf("expected home %s, got %s", home, got)
	}
}

func TestCommandRegistration(t *testing.T) {
	// Proxy service lifecycle lives under `devmesh proxy`
	wantProxy := map[string]bool{"start": false, "stop": false, "remove": false, "status": false}
	for _, sub := range proxyCmd.Commands() {
		if _, ok := wantProxy[sub.Name()]; ok {
			wantProxy[sub.Name()] = true
		}
	}
	for name, found := range wantProxy {
		if !found {
			t.Errorf("expected 'devmesh proxy %s' to be registered", name)
		}
	}

	// Tool management is top-level
	if installCmd.Parent() != rootCmd {
		t.Error("expected 'devmesh install' to be a top-level command")
	}
	if uninstallCmd.Parent() != rootCmd {
		t.Error("expected 'devmesh uninstall' to be a top-level command")
	}

	// Named project commands are top-level
	for _, c := range []*cobra.Command{startCmd, stopCmd, restartCmd} {
		if c.Parent() != rootCmd {
			t.Errorf("expected 'devmesh %s' to be a top-level command", c.Name())
		}
	}

	if keepDataFlag {
		_ = keepDataFlag // referenced to avoid unused warnings in short builds
	}
}
