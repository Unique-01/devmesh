package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Route represents a mapping from a hostname (e.g. vault.dev or *.dev) to a target backend URL.
type Route struct {
	Host   string // e.g., vault.dev, api.dev, or wildcard like *.dev
	Target *url.URL
}

// RouteRegistry stores routes and resolves hostnames to target backends.
type RouteRegistry struct {
	mu     sync.RWMutex
	routes map[string]*url.URL // exact host matches
	wildcards []wildcardRoute  // wildcard matches like *.dev
}

type wildcardRoute struct {
	suffix string // e.g. .dev or example.com
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
	// Strip port if present (e.g. vault.dev:8080 -> vault.dev)
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

// Server represents the HTTP reverse proxy server.
type Server struct {
	addr     string
	registry *RouteRegistry
	server   *http.Server
	transport *http.Transport
}

// NewServer creates a new reverse proxy server.
func NewServer(addr string, registry *RouteRegistry) *Server {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &Server{
		addr:      addr,
		registry:  registry,
		transport: transport,
	}
}

// Handler returns the http.Handler for the reverse proxy.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target, ok := s.registry.Resolve(r.Host)
		if !ok {
			http.Error(w, fmt.Sprintf("Bad Gateway: No route registered for hostname %q", r.Host), http.StatusBadGateway)
			return
		}

		proxy := &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Scheme = target.Scheme
				req.URL.Host = target.Host
				req.Host = target.Host // Preserve or set upstream host if needed, or keep original Host? Usually upstream target host or original. Let's set to target.Host or keep original depending on backend needs. Standard reverse proxy sets req.URL.Host and updates headers.
				
				// Standard headers
				if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
					if prior := req.Header.Get("X-Forwarded-For"); prior != "" {
						req.Header.Set("X-Forwarded-For", prior+", "+clientIP)
					} else {
						req.Header.Set("X-Forwarded-For", clientIP)
					}
				}
				req.Header.Set("X-Forwarded-Host", req.Host)
				req.Header.Set("X-Forwarded-Proto", "http") // or https if tls terminated
			},
			Transport: s.transport,
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				if errors.Is(err, context.Canceled) {
					return
				}
				http.Error(w, fmt.Sprintf("Bad Gateway: %v", err), http.StatusBadGateway)
			},
		}

		proxy.ServeHTTP(w, r)
	})
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	s.server = &http.Server{
		Addr:    s.addr,
		Handler: s.Handler(),
	}
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}
