package proxy

import (
	"fmt"
	"net"
)

// AllocatePort tries the preferred port first. If it is unavailable or empty,
// it allocates a free TCP port by letting the OS choose an available port.
// It returns the selected port number and any error encountered.
func AllocatePort(preferredPort int) (int, error) {
	if preferredPort > 0 {
		addr := fmt.Sprintf("127.0.0.1:%d", preferredPort)
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			port := listener.Addr().(*net.TCPAddr).Port
			listener.Close()
			return port, nil
		}
	}

	// Fallback to OS-assigned free port on localhost
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("failed to allocate free port: %w", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	return port, nil
}
