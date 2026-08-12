package proxy

import (
	"context"
	"net"
	"net/http"
	"time"
)

// Server represents the HTTP reverse proxy server.
type Server struct {
	addr      string
	registry  *RouteRegistry
	server    *http.Server
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
	return NewProxyHandler(s.registry, s.transport)
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
