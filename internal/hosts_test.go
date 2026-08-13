package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHostsManagerOperations(t *testing.T) {
	tmpDir := t.TempDir()
	hostsFile := filepath.Join(tmpDir, "hosts")

	// Initial hosts file with unrelated entries
	initialContent := "127.0.0.1\tlocalhost\n192.168.1.1\trouter.local\n"
	if err := os.WriteFile(hostsFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to write initial hosts file: %v", err)
	}

	hm := NewHostsManager(hostsFile)

	// 1. Add entry vault.dev
	if err := hm.AddEntry("vault.dev", "127.0.0.1"); err != nil {
		t.Fatalf("failed to add entry: %v", err)
	}

	content, err := os.ReadFile(hostsFile)
	if err != nil {
		t.Fatalf("failed to read hosts file: %v", err)
	}
	sContent := string(content)

	if !strings.Contains(sContent, "router.local") {
		t.Errorf("unrelated entries not preserved: %s", sContent)
	}
	if !strings.Contains(sContent, "vault.dev") {
		t.Errorf("added entry vault.dev missing: %s", sContent)
	}
	if !strings.Contains(sContent, devmeshHeaderBegin) || !strings.Contains(sContent, devmeshHeaderEnd) {
		t.Errorf("devmesh managed markers missing: %s", sContent)
	}

	// 2. Add duplicate entry (should handle gracefully without duplicating)
	if err := hm.AddEntry("vault.dev", "127.0.0.1"); err != nil {
		t.Fatalf("failed to add duplicate entry: %v", err)
	}

	content, _ = os.ReadFile(hostsFile)
	count := strings.Count(string(content), "vault.dev")
	if count != 1 {
		t.Errorf("expected exactly 1 instance of vault.dev after duplicate add, got %d in:\n%s", count, string(content))
	}

	// 3. Add another entry shop.dev
	if err := hm.AddEntry("shop.dev", "127.0.0.1"); err != nil {
		t.Fatalf("failed to add second entry: %v", err)
	}

	// 4. Remove entry vault.dev
	if err := hm.RemoveEntry("vault.dev"); err != nil {
		t.Fatalf("failed to remove entry: %v", err)
	}

	content, _ = os.ReadFile(hostsFile)
	sContent = string(content)
	if strings.Contains(sContent, "vault.dev") {
		t.Errorf("expected vault.dev to be removed, found in:\n%s", sContent)
	}
	if !strings.Contains(sContent, "shop.dev") {
		t.Errorf("expected shop.dev to be preserved, missing in:\n%s", sContent)
	}
	if !strings.Contains(sContent, "router.local") {
		t.Errorf("expected unrelated entry router.local to be preserved, missing in:\n%s", sContent)
	}
}

func TestHostsManagerPermissionFailure(t *testing.T) {
	tmpDir := t.TempDir()
	hostsFile := filepath.Join(tmpDir, "readonly_hosts")

	// Create file with 000 permissions (no read/write)
	if err := os.WriteFile(hostsFile, []byte("127.0.0.1 localhost\n"), 0000); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	defer os.Chmod(hostsFile, 0644) // restore for cleanup

	hm := NewHostsManager(hostsFile)

	err := hm.AddEntry("vault.dev", "127.0.0.1")
	if err == nil {
		t.Error("expected permission error when writing to read-restricted file, got nil")
	} else if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("expected permission denied error message, got: %v", err)
	}
}
