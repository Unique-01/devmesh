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

	// Test explicit --name vault, --domain api.localhost
	ident, err = ResolveIdentity("vault", "api.localhost", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ident.Name != "vault" || ident.Domain != "api.localhost" {
		t.Errorf("expected vault / api.localhost, got %s / %s", ident.Name, ident.Domain)
	}

	// Test config name and domain
	ident, err = ResolveIdentity("", "", "shop", "shop.localhost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ident.Name != "shop" || ident.Domain != "shop.localhost" {
		t.Errorf("expected shop / shop.localhost, got %s / %s", ident.Name, ident.Domain)
	}

	// Test override: explicit flags take precedence over config
	ident, err = ResolveIdentity("override-name", "override.localhost", "shop", "shop.localhost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ident.Name != "override-name" || ident.Domain != "override.localhost" {
		t.Errorf("expected override-name / override.localhost, got %s / %s", ident.Name, ident.Domain)
	}
}
