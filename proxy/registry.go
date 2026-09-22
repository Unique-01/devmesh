package proxy

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
)

// Route represents a mapping from a hostname (e.g. vault.localhost or *.localhost) to a target backend URL.
type Route struct {
	Host   string // e.g., vault.localhost, api.localhost, or wildcard like *.localhost
	Target *url.URL
}

// RouteRegistry stores routes and resolves hostnames to target backends.
type RouteRegistry struct {
	mu        sync.RWMutex
	routes    map[string]*url.URL // exact host matches
	wildcards []wildcardRoute     // wildcard matches like *.localhost
}

type wildcardRoute struct {
	suffix string // e.g. .localhost or example.com
	target *url.URL
}

// NewRouteRegistry creates a new empty RouteRegistry.
func NewRouteRegistry() *RouteRegistry {
	return &RouteRegistry{
		routes: make(map[string]*url.URL),
	}
}

// AddRoute adds or updates a route for a given hostname and target URL string.
func (r *RouteRegistry) AddRoute(host string, targetStr string) error {
	target, err := url.Parse(targetStr)
	if err != nil {
		return fmt.Errorf("invalid target URL: %w", err)
	}

	host = strings.ToLower(strings.TrimSpace(host))

	r.mu.Lock()
	defer r.mu.Unlock()

	if strings.HasPrefix(host, "*.") {
		suffix := strings.TrimPrefix(host, "*")
		r.wildcards = append(r.wildcards, wildcardRoute{
			suffix: suffix,
			target: target,
		})
	} else {
		r.routes[host] = target
	}
	return nil
}

// GetRoutes returns a copy of all routes as host -> target strings.
func (r *RouteRegistry) GetRoutes() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]string, len(r.routes)+len(r.wildcards))
	for k, v := range r.routes {
		result[k] = v.String()
	}
	for _, w := range r.wildcards {
		result["*"+w.suffix] = w.target.String()
	}
	return result
}

// RemoveRoute removes a route by hostname.
func (r *RouteRegistry) RemoveRoute(host string) {
	host = strings.ToLower(strings.TrimSpace(host))
	r.mu.Lock()
	defer r.mu.Unlock()

	if strings.HasPrefix(host, "*.") {
		suffix := strings.TrimPrefix(host, "*")
		newWildcards := make([]wildcardRoute, 0, len(r.wildcards))
		for _, w := range r.wildcards {
			if w.suffix != suffix {
				newWildcards = append(newWildcards, w)
			}
		}
		r.wildcards = newWildcards
	} else {
		delete(r.routes, host)
	}
}

// Resolve looks up a hostname and returns the target URL if found.
func (r *RouteRegistry) Resolve(host string) (*url.URL, bool) {
	// Strip port if present (e.g. vault.localhost:8080 -> vault.localhost)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.ToLower(strings.TrimSpace(host))

	r.mu.RLock()
	defer r.mu.RUnlock()

	// 1. Exact match
	if target, ok := r.routes[host]; ok {
		return target, true
	}

	// 2. Wildcard match
	for _, w := range r.wildcards {
		if strings.HasSuffix(host, w.suffix) {
			return w.target, true
		}
	}

	return nil, false
}
