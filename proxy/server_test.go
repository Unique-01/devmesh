package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouteRegistry(t *testing.T) {
	reg := NewRouteRegistry()

	// Add exact route
	err := reg.AddRoute("vault.dev", "http://localhost:3000")
	if err != nil {
		t.Fatalf("failed to add route: %v", err)
	}

	// Add wildcard route
	err = reg.AddRoute("*.example.com", "http://localhost:4000")
	if err != nil {
		t.Fatalf("failed to add wildcard route: %v", err)
	}

	// Test exact match
	target, ok := reg.Resolve("vault.dev")
	if !ok || target.String() != "http://localhost:3000" {
		t.Errorf("expected vault.dev to resolve to http://localhost:3000, got %v (ok=%v)", target, ok)
	}

	// Test case insensitivity and port stripping
	target, ok = reg.Resolve("VAULT.DEV:8080")
	if !ok || target.String() != "http://localhost:3000" {
		t.Errorf("expected VAULT.DEV:8080 to resolve to http://localhost:3000, got %v (ok=%v)", target, ok)
	}

	// Test wildcard match
	target, ok = reg.Resolve("app.example.com")
	if !ok || target.String() != "http://localhost:4000" {
		t.Errorf("expected app.example.com to resolve to http://localhost:4000, got %v (ok=%v)", target, ok)
	}

	// Test unknown route
	_, ok = reg.Resolve("unknown.dev")
	if ok {
		t.Errorf("expected unknown.dev not to resolve")
	}

	// Test removal
	reg.RemoveRoute("vault.dev")
	_, ok = reg.Resolve("vault.dev")
	if ok {
		t.Errorf("expected removed route vault.dev not to resolve")
	}
}

func TestReverseProxyServer(t *testing.T) {
	// 1. Start a test backend server representing localhost:3000
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from backend! Path: %s, Host: %s", r.URL.Path, r.Host)
	}))
	defer backend.Close()

	// 2. Set up route registry
	reg := NewRouteRegistry()
	err := reg.AddRoute("vault.dev", backend.URL)
	if err != nil {
		t.Fatalf("failed to add route: %v", err)
	}

	// 3. Create proxy server
	proxyServer := NewServer("127.0.0.1:0", reg)

	// 4. Create httptest server using proxy handler
	ts := httptest.NewServer(proxyServer.Handler())
	defer ts.Close()

	// 5. Make request simulating vault.dev host header hitting the proxy
	client := ts.Client()
	req, err := http.NewRequest("GET", ts.URL+"/test-path", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Host = "vault.dev"

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	bodyStr := string(bodyBytes)
	expectedBody := fmt.Sprintf("Hello from backend! Path: /test-path, Host: %s", backend.Listener.Addr().String())
	if bodyStr != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, bodyStr)
	}

	// 6. Test unknown host
	reqUnknown, err := http.NewRequest("GET", ts.URL+"/test-path", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	reqUnknown.Host = "unknown.dev"

	respUnknown, err := client.Do(reqUnknown)
	if err != nil {
		t.Fatalf("failed to execute request: %v", err)
	}
	defer respUnknown.Body.Close()

	if respUnknown.StatusCode != http.StatusBadGateway {
		t.Errorf("expected status 502 for unknown host, got %d", respUnknown.StatusCode)
	}
}
