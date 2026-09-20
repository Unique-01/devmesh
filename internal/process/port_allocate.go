package process

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
)

// AllocatePort checks the user's intended preferredPort. If available, it uses it.
// If preferredPort is busy or 0, it allocates a random available TCP port accessible for non-root users (1024-65535).
func AllocatePort(preferredPort int) (int, error) {
	if preferredPort > 0 {
		addr := fmt.Sprintf("127.0.0.1:%d", preferredPort)
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			listener.Close()
			return preferredPort, nil
		}
		fmt.Printf("Intended port %d is already in use. Falling back to random available port...\n", preferredPort)
	}

	// Fallback / default when no port specified or preferred port is unavailable:
	// Use any random unprivileged port (1024-65535)
	for range 50 {
		randPort, err := randomUnprivilegedPort()
		if err != nil {
			continue
		}
		addr := fmt.Sprintf("127.0.0.1:%d", randPort)
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			listener.Close()
			return randPort, nil
		}
	}

	// Last resort: OS assigned port (:0)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("failed to allocate free port: %w", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	return port, nil
}

func randomUnprivilegedPort() (int, error) {
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	val := binary.BigEndian.Uint16(b[:])
	// Range: 1024 to 65535
	port := 1024 + int(val)%(65535-1024+1)
	return port, nil
}
