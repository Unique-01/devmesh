package proxy

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouteRegistry(t *testing.T) {
	reg := NewRouteRegistry()

	// Add exact route
	err := reg.AddRoute("vault.localhost", "http://localhost:3000")
	if err != nil {
		t.Fatalf("failed to add route: %v", err)
	}

	// Add wildcard route
	err = reg.AddRoute("*.example.com", "http://localhost:4000")
	if err != nil {
		t.Fatalf("failed to add wildcard route: %v", err)
	}

	// Test exact match
	target, ok := reg.Resolve("vault.localhost")
	if !ok || target.String() != "http://localhost:3000" {
		t.Errorf("expected vault.localhost to resolve to http://localhost:3000, got %v (ok=%v)", target, ok)
	}

	// Test case insensitivity and port stripping
	target, ok = reg.Resolve("VAULT.LOCALHOST:8080")
	if !ok || target.String() != "http://localhost:3000" {
		t.Errorf("expected VAULT.LOCALHOST:8080 to resolve to http://localhost:3000, got %v (ok=%v)", target, ok)
	}

	// Test wildcard match
	target, ok = reg.Resolve("app.example.com")
	if !ok || target.String() != "http://localhost:4000" {
		t.Errorf("expected app.example.com to resolve to http://localhost:4000, got %v (ok=%v)", target, ok)
	}

	// Test unknown route
	_, ok = reg.Resolve("unknown.localhost")
	if ok {
		t.Errorf("expected unknown.localhost not to resolve")
	}

	// Test removal
	reg.RemoveRoute("vault.localhost")
	_, ok = reg.Resolve("vault.localhost")
	if ok {
		t.Errorf("expected removed route vault.localhost not to resolve")
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
	err := reg.AddRoute("vault.localhost", backend.URL)
	if err != nil {
		t.Fatalf("failed to add route: %v", err)
	}

	// 3. Create proxy server
	proxyServer := NewServer("127.0.0.1:0", reg)

	// 4. Create httptest server using proxy handler
	ts := httptest.NewServer(proxyServer.Handler())
	defer ts.Close()

	// 5. Make request simulating vault.localhost host header hitting the proxy
	client := ts.Client()
	req, err := http.NewRequest("GET", ts.URL+"/test-path", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Host = "vault.localhost"

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
	reqUnknown.Host = "unknown.localhost"

	respUnknown, err := client.Do(reqUnknown)
	if err != nil {
		t.Fatalf("failed to execute request: %v", err)
	}
	defer respUnknown.Body.Close()

	if respUnknown.StatusCode != http.StatusBadGateway {
		t.Errorf("expected status 502 for unknown host, got %d", respUnknown.StatusCode)
	}
}

// TestReverseProxyServer_IPv6OnlyBackend replicates Node 17+ / Vite behavior:
// the app listens on ::1 only (localhost resolves to ::1 first on many
// systems), so routes must be registered via http://localhost:<port> — the Go
// dialer then falls back across resolved families. A 127.0.0.1 target would
// get connection refused here.
func TestReverseProxyServer_IPv6OnlyBackend(t *testing.T) {
	l, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback not available: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port

	backendSrv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "hello from ::1 only")
	})}
	go backendSrv.Serve(l)
	defer backendSrv.Close()

	reg := NewRouteRegistry()
	if err := reg.AddRoute("vault.localhost", fmt.Sprintf("http://localhost:%d", port)); err != nil {
		t.Fatalf("failed to add route: %v", err)
	}

	proxyServer := NewServer("127.0.0.1:0", reg)
	ts := httptest.NewServer(proxyServer.Handler())
	defer ts.Close()

	req, err := http.NewRequest("GET", ts.URL+"/", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Host = "vault.localhost"

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if string(body) != "hello from ::1 only" {
		t.Errorf("expected body from IPv6-only backend, got %q", string(body))
	}
}
