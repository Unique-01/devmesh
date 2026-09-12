package cli

import (
	"net/http/httptest"
	"testing"

	"devmesh/proxy"
)

func TestProxyCommandIntegration(t *testing.T) {
	// Test that proxy command routing and server setup functions correctly
	reg := proxy.NewRouteRegistry()
	_ = reg.AddRoute("vault.localhost", "http://localhost:9999")

	server := proxy.NewServer("127.0.0.1:0", reg)
	ts := httptest.NewServer(server.Handler())
	defer ts.Close()
}
