package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// RegisterRoute sends a request to the proxy daemon to register a new route.
func RegisterRoute(proxyAddr, domain, target string) error {
	url := fmt.Sprintf("http://%s/_devmesh/routes", proxyAddr)
	payload := map[string]string{"domain": domain, "target": target}
	data, _ := json.Marshal(payload)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("proxy returned status %d", resp.StatusCode)
	}
	return nil
}

// DeregisterRoute sends a request to the proxy daemon to remove a route.
func DeregisterRoute(proxyAddr, domain string) error {
	url := fmt.Sprintf("http://%s/_devmesh/routes?domain=%s", proxyAddr, domain)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("proxy returned status %d", resp.StatusCode)
	}
	return nil
}
