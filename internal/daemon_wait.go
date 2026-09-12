package internal

import (
	"fmt"
	"net/http"
	"time"
)

// PingDaemon does a single quick probe of the proxy daemon's admin endpoint.
func PingDaemon(url string) bool {
	client := &http.Client{Timeout: 300 * time.Millisecond}
	resp, err := client.Get(url + "/_devmesh/ping")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// WaitForDaemon pings the proxy until it's ready or times out.
func WaitForDaemon(url string) error {
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timed out waiting for proxy daemon at %s", url)
		case <-ticker.C:
			resp, err := http.Get(url + "/_devmesh/ping")
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				return nil
			}
		}
	}
}
