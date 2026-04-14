package server

import (
	"encoding/json"
	"net/http"

	"connectrpc.com/connect"

	"github.com/nebari-dev/skillsctl/backend/internal/auth"
	"github.com/nebari-dev/skillsctl/backend/internal/registry"
	"github.com/nebari-dev/skillsctl/backend/internal/store"
	"github.com/nebari-dev/skillsctl/gen/go/skillsctl/v1/skillsctlv1connect"
	"github.com/nebari-dev/skillsctl/internal/skillpkg"
)

// Server is the main HTTP server that mounts the health check and ConnectRPC handlers.
type Server struct {
	handler http.Handler
}

// Option is a functional option for configuring a Server.
type Option func(*serverConfig)

type serverConfig struct {
	limits skillpkg.Limits
}

// WithLimits sets the resource limits exposed by the GET /limits endpoint.
func WithLimits(l skillpkg.Limits) Option {
	return func(c *serverConfig) { c.limits = l }
}

// New creates a Server wired to the given skill store with optional auth.
// If authValidator is nil, authentication is disabled (local dev mode).
func New(skillStore store.Repository, authValidator auth.TokenValidator, authCfg auth.Config, opts ...Option) *Server {
	cfg := serverConfig{limits: skillpkg.DefaultLimits()}
	for _, o := range opts {
		o(&cfg)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealthz)
	mux.HandleFunc("/auth/config", handleAuthConfig(authCfg))
	mux.HandleFunc("/limits", handleLimits(cfg.limits))

	interceptor := auth.NewInterceptor(authValidator)
	path, handler := skillsctlv1connect.NewRegistryServiceHandler(
		registry.NewService(skillStore),
		connect.WithInterceptors(interceptor),
	)
	mux.Handle(path, handler)

	wrapped := auth.NewAllowlistMiddleware([]string{"/healthz", "/auth/config", "/limits"}, mux)
	return &Server{handler: wrapped}
}

type authConfigResponse struct {
	Enabled        bool   `json:"enabled"`
	IssuerURL      string `json:"issuer_url,omitempty"`
	ClientID       string `json:"client_id,omitempty"`
	DeviceClientID string `json:"device_client_id,omitempty"`
}

func handleAuthConfig(cfg auth.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		resp := authConfigResponse{
			Enabled: cfg.IssuerURL != "" && cfg.ClientID != "",
		}
		if resp.Enabled {
			resp.IssuerURL = cfg.IssuerURL
			resp.ClientID = cfg.ClientID
			resp.DeviceClientID = cfg.DeviceClientID
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
