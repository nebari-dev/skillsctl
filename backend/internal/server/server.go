package server

import (
	"net/http"

	"github.com/openteams-ai/skill-share/backend/internal/registry"
	"github.com/openteams-ai/skill-share/backend/internal/store"
	"github.com/openteams-ai/skill-share/gen/go/skillctl/v1/skillctlv1connect"
)

// Server is the main HTTP server that mounts the health check and ConnectRPC handlers.
type Server struct {
	mux *http.ServeMux
}

// New creates a Server wired to the given skill store.
func New(skillStore store.SkillStore) *Server {
	s := &Server{
		mux: http.NewServeMux(),
	}
	s.mux.HandleFunc("/healthz", s.handleHealthz)

	registrySvc := registry.NewService(skillStore)
	path, handler := skillctlv1connect.NewRegistryServiceHandler(registrySvc)
	s.mux.Handle(path, handler)

	return s
}

// Handler returns the http.Handler for this server.
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
