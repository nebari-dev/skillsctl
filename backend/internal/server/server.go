package server

import (
	"net/http"

	"connectrpc.com/connect"

	"github.com/nebari-dev/skillctl/backend/internal/auth"
	"github.com/nebari-dev/skillctl/backend/internal/registry"
	"github.com/nebari-dev/skillctl/backend/internal/store"
	"github.com/nebari-dev/skillctl/gen/go/skillctl/v1/skillctlv1connect"
)

// Server is the main HTTP server that mounts the health check and ConnectRPC handlers.
type Server struct {
	handler http.Handler
}

// New creates a Server wired to the given skill store with optional auth.
// If authValidator is nil, authentication is disabled (local dev mode).
func New(skillStore store.Repository, authValidator auth.TokenValidator) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealthz)

	interceptor := auth.NewInterceptor(authValidator)
	path, handler := skillctlv1connect.NewRegistryServiceHandler(
		registry.NewService(skillStore),
		connect.WithInterceptors(interceptor),
	)
	mux.Handle(path, handler)

	wrapped := auth.NewAllowlistMiddleware([]string{"/healthz"}, mux)
	return &Server{handler: wrapped}
}

// Handler returns the http.Handler for this server.
func (s *Server) Handler() http.Handler {
	return s.handler
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
