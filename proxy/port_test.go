package proxy

import (
	"net"
	"testing"
)

func TestAllocatePort_PreferredAvailable(t *testing.T) {
	// Find a free port first to use as preferred
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	prefPort := l.Addr().(*net.TCPAddr).Port
	l.Close() // Release it so it's available

	port, err := AllocatePort(prefPort)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if port != prefPort {
		t.Errorf("expected preferred port %d, got %d", prefPort, port)
	}
}

func TestAllocatePort_PreferredUnavailableFallback(t *testing.T) {
	// Occupy a port
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer l.Close()
	prefPort := l.Addr().(*net.TCPAddr).Port

	// Try allocating with preferred port already in use
	port, err := AllocatePort(prefPort)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if port == prefPort {
		t.Errorf("expected fallback port different from unavailable preferred port %d, got %d", prefPort, port)
	}
	if port <= 0 {
		t.Errorf("expected valid allocated port, got %d", port)
	}
}

func TestAllocatePort_ZeroPreferred(t *testing.T) {
	port, err := AllocatePort(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port <= 0 {
		t.Errorf("expected valid allocated port, got %d", port)
	}
}
