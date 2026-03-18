package server

import (
	"encoding/json"
	"net/http"

	"connectrpc.com/connect"

	"github.com/nebari-dev/skillsctl/backend/internal/auth"
	"github.com/nebari-dev/skillsctl/backend/internal/registry"
	"github.com/nebari-dev/skillsctl/backend/internal/store"
	"github.com/nebari-dev/skillsctl/gen/go/skillsctl/v1/skillsctlv1connect"
)

// Server is the main HTTP server that mounts the health check and ConnectRPC handlers.
type Server struct {
	handler http.Handler
}

// New creates a Server wired to the given skill store with optional auth.
// If authValidator is nil, authentication is disabled (local dev mode).
func New(skillStore store.Repository, authValidator auth.TokenValidator, authCfg auth.Config) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealthz)
	mux.HandleFunc("/auth/config", handleAuthConfig(authCfg))

	interceptor := auth.NewInterceptor(authValidator)
	path, handler := skillsctlv1connect.NewRegistryServiceHandler(
		registry.NewService(skillStore),
		connect.WithInterceptors(interceptor),
	)
	mux.Handle(path, handler)

	wrapped := auth.NewAllowlistMiddleware([]string{"/healthz", "/auth/config"}, mux)
	return &Server{handler: wrapped}
}

type authConfigResponse struct {
	Enabled   bool   `json:"enabled"`
	IssuerURL string `json:"issuer_url,omitempty"`
	ClientID  string `json:"client_id,omitempty"`
}

func handleAuthConfig(cfg auth.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		resp := authConfigResponse{
			Enabled: cfg.IssuerURL != "" && cfg.ClientID != "",
		}
		if resp.Enabled {
			resp.IssuerURL = cfg.IssuerURL
			resp.ClientID = cfg.ClientID
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// Handler returns the http.Handler for this server.
func (s *Server) Handler() http.Handler {
	return s.handler
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
