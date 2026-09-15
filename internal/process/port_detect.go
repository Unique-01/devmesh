package process

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func detectPort(ctx context.Context, pid int, portChan <-chan int, onPortDetected func(port int)) {
	// Discover the port the child actually bound. Primary: parse startup
	// output (URL/port patterns). Fallback: poll listening TCP sockets of the
	// child's whole process group (works for servers that print no URL).
	if onPortDetected == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(400 * time.Millisecond)
		defer ticker.Stop()
		timeout := time.After(30 * time.Second)

		for {
			select {
			case <-ctx.Done():
				return
			case <-timeout:
				return
			case p := <-portChan:
				if p > 0 {
					onPortDetected(p)
					return
				}
			case <-ticker.C:
				if p := detectPortViaSocketPoll(pid); p > 0 {
					onPortDetected(p)
					return
				}
			}
		}
	}()
}

var (
	reAddrInUse = regexp.MustCompile(`(?i)eaddrinuse|address already in use|only one usage of each socket address`)
	// Lines saying a port is busy/taken are NOT the port the server bound on
	// (e.g. Vite's "Port 5173 is in use, trying another one...").
	reBusyPort = regexp.MustCompile(`(?i)\bin use\b|\bbusy\b|trying another|already in use`)
	reURLPort  = regexp.MustCompile(`(?:localhost|127\.0\.0\.1|0\.0\.0\.0|\*|\[::1\]):(\d{2,5})`)
	rePortWord = regexp.MustCompile(`(?i)\bport["']?\s*[:=]\s*["']?(\d{2,5})\b`)
)

// scanStreamForPort scans a child's output stream for the port it bound and
// for bind failures, delivering results on the provided channels.
func scanStreamForPort(r io.Reader, portChan chan<- int, addrInUseChan chan<- struct{}) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()

		if reAddrInUse.MatchString(line) {
			select {
			case addrInUseChan <- struct{}{}:
			default:
			}
			continue
		}
		if reBusyPort.MatchString(line) {
			continue
		}

		port := 0
		if m := reURLPort.FindStringSubmatch(line); len(m) >= 2 {
			port, _ = strconv.Atoi(m[1])
		}
		if port == 0 {
			if m := rePortWord.FindStringSubmatch(line); len(m) >= 2 {
				port, _ = strconv.Atoi(m[1])
			}
		}
		if port >= 1024 && port <= 65535 {
			select {
			case portChan <- port:
			default:
			}
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "devmesh: output scan error: %v\n", err)
	}
}

// detectPortViaSocketPoll finds a TCP port in LISTEN state owned by any
// process in the given process group. The child runs in its own process group
// (Setpgid), so inspecting the group covers the shell wrapper plus the actual
// server process (e.g. sh -> pnpm -> vite), which a plain `lsof -p <pid>`
// would miss.
func detectPortViaSocketPoll(pgid int) int {
	if pgid <= 0 {
		return 0
	}
	out, err := exec.Command("lsof", "-a", "-iTCP", "-sTCP:LISTEN", "-P", "-n", "-g", strconv.Itoa(pgid)).Output()
	if err != nil {
		return 0
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		if strings.HasPrefix(line, "COMMAND") {
			continue
		}
		line = strings.ReplaceAll(line, "(LISTEN)", "")
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := fields[len(fields)-1]
		idx := strings.LastIndex(name, ":")
		if idx == -1 {
			continue
		}
		if p, err := strconv.Atoi(name[idx+1:]); err == nil && p >= 1024 && p <= 65535 {
			return p
		}
	}
	return 0
}
