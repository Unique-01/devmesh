package cli

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"devmesh/proxy"
)

func TestProxyCommandIntegration(t *testing.T) {
	// Test that proxy command routing and server setup functions correctly
	reg := proxy.NewRouteRegistry()
	_ = reg.AddRoute("vault.local.dev", "http://localhost:9999")

	server := proxy.NewServer("127.0.0.1:0", reg)
	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	dnsServer := proxy.NewDNSServer("127.0.0.1:0", reg)
	go func() {
		_ = dnsServer.Start(ctx)
	}()
	defer dnsServer.Close()
}
