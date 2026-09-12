package process

import (
	"strings"
	"testing"
)

func scanLines(t *testing.T, lines string) (ports []int, addrInUse bool) {
	t.Helper()
	portChan := make(chan int, 8)
	addrInUseChan := make(chan struct{}, 1)
	scanStreamForPort(strings.NewReader(lines), portChan, addrInUseChan)
	close(portChan)
	for p := range portChan {
		ports = append(ports, p)
	}
	select {
	case <-addrInUseChan:
		addrInUse = true
	default:
	}
	return ports, addrInUse
}

func TestScanStreamForPort_DetectsURLPort(t *testing.T) {
	ports, _ := scanLines(t, "  ➜  Local:   http://localhost:5173/\n")
	if len(ports) != 1 || ports[0] != 5173 {
		t.Errorf("expected [5173], got %v", ports)
	}
}

func TestScanStreamForPort_DetectsPortWord(t *testing.T) {
	ports, _ := scanLines(t, "INFO: Server started.\n    port: \"32372\"\n")
	if len(ports) != 1 || ports[0] != 32372 {
		t.Errorf("expected [32372], got %v", ports)
	}
}

func TestScanStreamForPort_IgnoresBusyPortMessage(t *testing.T) {
	// Vite announces the busy port before retrying; the busy port must NOT be
	// picked up, but the retry port (in the URL) must be.
	ports, _ := scanLines(t, "Port 5173 is in use, trying another one...\n  ➜  Local:   http://localhost:5174/\n")
	if len(ports) != 1 || ports[0] != 5174 {
		t.Errorf("expected [5174], got %v", ports)
	}
}

func TestScanStreamForPort_DetectsAddrInUse(t *testing.T) {
	_, addrInUse := scanLines(t, "Error: listen EADDRINUSE: address already in use 127.0.0.1:3000\n")
	if !addrInUse {
		t.Error("expected addr-in-use to be detected")
	}
}

func TestScanStreamForPort_IgnoresNonListenOutput(t *testing.T) {
	ports, addrInUse := scanLines(t, "VITE v8.2.2  ready in 316 ms\n➜  Network: use --host to expose\n")
	if len(ports) != 0 || addrInUse {
		t.Errorf("expected no detections, got ports=%v addrInUse=%v", ports, addrInUse)
	}
}

func TestScanStreamForPort_IgnoresPrivilegedAndInvalidPorts(t *testing.T) {
	ports, _ := scanLines(t, "listening on http://localhost:80\nfoo http://localhost:99999\n")
	if len(ports) != 0 {
		t.Errorf("expected no ports, got %v", ports)
	}
}
