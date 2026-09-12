package proxy

import (
	"testing"
)

func TestActiveRouteManager(t *testing.T) {
	reg := NewRouteRegistry()
	mgr := NewActiveRouteManager(reg)

	domain := "vault.localhost"
	target := "http://127.0.0.1:43127"

	if err := mgr.RegisterRoute(domain, target); err != nil {
		t.Fatalf("failed to register route: %v", err)
	}

	routes := mgr.GetRoutes()
	if routes[domain] != target {
		t.Errorf("expected route %s -> %s, got %+v", domain, target, routes)
	}

	resolved, ok := reg.Resolve(domain)
	if !ok || resolved.String() != target {
		t.Errorf("expected registry to resolve %s to %s, got %v (ok=%v)", domain, target, resolved, ok)
	}

	mgr.UnregisterRoute(domain)
	routes = mgr.GetRoutes()
	if _, ok := routes[domain]; ok {
		t.Errorf("expected route %s to be unregistered", domain)
	}

	_, ok = reg.Resolve(domain)
	if ok {
		t.Errorf("expected registry not to resolve unregistered domain %s", domain)
	}
}
