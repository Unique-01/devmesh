package internal

import (
	"testing"
)

func TestResolveIdentity(t *testing.T) {
	// Test folder name fallback (when no explicit flags or config provided)
	// We can test ResolveIdentity directly.
	ident, err := ResolveIdentity("", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ident.Name == "" || ident.Domain == "" {
		t.Errorf("expected non-empty name and domain, got %+v", ident)
	}

	// Test explicit --name vault, --domain api.dev
	ident, err = ResolveIdentity("vault", "api.dev", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ident.Name != "vault" || ident.Domain != "api.dev" {
		t.Errorf("expected vault / api.dev, got %s / %s", ident.Name, ident.Domain)
	}

	// Test config name and domain
	ident, err = ResolveIdentity("", "", "shop", "shop.dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ident.Name != "shop" || ident.Domain != "shop.dev" {
		t.Errorf("expected shop / shop.dev, got %s / %s", ident.Name, ident.Domain)
	}

	// Test override: explicit flags take precedence over config
	ident, err = ResolveIdentity("override-name", "override.dev", "shop", "shop.dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ident.Name != "override-name" || ident.Domain != "override.dev" {
		t.Errorf("expected override-name / override.dev, got %s / %s", ident.Name, ident.Domain)
	}
}
