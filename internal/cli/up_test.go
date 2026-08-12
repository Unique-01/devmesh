package cli

import (
	"testing"
)

func TestUpCommandRequiresCmd(t *testing.T) {
	cmdFlag = ""
	err := upCmd.RunE(upCmd, []string{})
	if err == nil {
		t.Error("expected error when --cmd is not specified, got nil")
	}
}
