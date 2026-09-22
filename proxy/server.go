package proxy

import (
	"context"
	"encoding/json"
	"net/http"
)

// Server represents the HTTP reverse proxy server.
type Server struct {
	addr      string
	registry  *RouteRegistry
	server    *http.Server
	transport *http.Transport
}

// AdminHandler handles internal devmesh administrative tasks.
func (s *Server) AdminHandler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/_devmesh/ping":
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "devmesh-proxy"})
	case "/_devmesh/routes":
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(s.registry.GetRoutes())
		case http.MethodPost:
			var req struct {
				Domain string `json:"domain"`
				Target string `json:"target"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := s.registry.AddRoute(req.Domain, req.Target); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			domain := r.URL.Query().Get("domain")
			s.registry.RemoveRoute(domain)
			w.WriteHeader(http.StatusOK)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	default:
		http.NotFound(w, r)
	}
}

// NewServer creates a new reverse proxy server.
func NewServer(addr string, registry *RouteRegistry) *Server {
	return &Server{
		addr:      addr,
		registry:  registry,
		transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
	}
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/_devmesh/", s.AdminHandler)
	mux.Handle("/", NewProxyHandler(s.registry, s.transport))
	return mux
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
