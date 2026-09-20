package internal

import (
	"testing"
)

func TestResolveIdentity(t *testing.T) {
	// Test folder name fallback (when no explicit flags or config provided)
	// We can test ResolveIdentity directly.
	ident, err := ResolveIdentity("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ident.Name == "" {
		t.Errorf("expected non-empty name, got %+v", ident)
	}

	// Test explicit --name vault
	ident, err = ResolveIdentity("vault", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ident.Name != "vault" || ident.Domain != "vault.localhost" {
		t.Errorf("expected vault / vault.localhost, got %s / %s", ident.Name, ident.Domain)
	}

	// Test config name and domain
	ident, err = ResolveIdentity("", "shop")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ident.Name != "shop" || ident.Domain != "shop.localhost" {
		t.Errorf("expected shop / shop.localhost, got %s / %s", ident.Name, ident.Domain)
	}

	// Test override: explicit flags take precedence over config
	ident, err = ResolveIdentity("override-name", "shop")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ident.Name != "override-name" || ident.Domain != "override-name.localhost" {
		t.Errorf("expected override-name / override-name.localhost, got %s / %s", ident.Name, ident.Domain)
	}
}
