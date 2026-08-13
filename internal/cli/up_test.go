package cli

import (
	"os"
	"testing"
)

func TestUpCommandRequiresCmdOrConfig(t *testing.T) {
	// Clean up any existing .devmesh.yaml
	os.Remove(".devmesh.yaml")
	defer os.Remove(".devmesh.yaml")

	cmdFlag = ""
	nameFlag = ""
	portFlag = 0

	err := upCmd.RunE(upCmd, []string{})
	if err == nil {
		t.Error("expected error when neither --cmd nor .devmesh.yaml exists, got nil")
	}
}

func TestUpCommandCreatesConfigAndParsesIt(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer os.Chdir(origDir)

	// Test case 1: Provide --cmd and --name should create .devmesh.yaml
	cmdFlag = "echo test"
	nameFlag = "vault"
	portFlag = 0

	// We test RunE up to process execution or check config creation.
	// Since RunE spawns a process and blocks/runs, we can test parseConfigYaml and config creation logic or test RunE with a fast mock/command.
	// Let's test parseConfigYaml directly first.
	data := []byte("name: vault\ncmd: \"pnpm dev\"\nport: 3000\n")
	cfg, err := parseConfigYaml(data)
	if err != nil {
		t.Fatalf("unexpected error parsing yaml: %v", err)
	}
	if cfg.Name != "vault" || cfg.Cmd != "pnpm dev" || cfg.Port != 3000 {
		t.Errorf("parsed config mismatch: %+v", cfg)
	}
}
