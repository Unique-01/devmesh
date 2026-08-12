package cli

import (
	"testing"
)

func TestRootCommand(t *testing.T) {
	cmd := rootCmd
	if cmd.Use != "devmesh" {
		t.Errorf("Expected root command use to be 'devmesh', got '%s'", cmd.Use)
	}
}
