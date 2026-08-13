package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
)

// NewProxyHandler creates an http.Handler that reverses proxy requests using the RouteRegistry and transport.
func NewProxyHandler(registry *RouteRegistry, transport *http.Transport) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target, ok := registry.Resolve(r.Host)
		if !ok {
			http.Error(w, fmt.Sprintf("Bad Gateway: No route registered for hostname %q", r.Host), http.StatusBadGateway)
			return
		}

		proxy := &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Scheme = target.Scheme
				req.URL.Host = target.Host
				req.Host = target.Host

				// Standard headers
				if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
					if prior := req.Header.Get("X-Forwarded-For"); prior != "" {
						req.Header.Set("X-Forwarded-For", prior+", "+clientIP)
					} else {
						req.Header.Set("X-Forwarded-For", clientIP)
					}
				}
				req.Header.Set("X-Forwarded-Host", req.Host)
				req.Header.Set("X-Forwarded-Proto", "http")
			},
			Transport: transport,
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				if errors.Is(err, context.Canceled) {
					return
				}
				http.Error(w, fmt.Sprintf("Bad Gateway: %v", err), http.StatusBadGateway)
			},
		}

		// Ensure WebSocket upgrades are properly handled (automatic in httputil.ReverseProxy 1.12+)
		proxy.ServeHTTP(w, r)
	})
}
