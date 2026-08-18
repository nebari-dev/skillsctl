package server

import (
	"encoding/json"
	"net/http"

	"github.com/nebari-dev/skillsctl/backend/internal/seed"
)

// SeedStatus reports what the startup seed did, so an operator can tell whether
// a release actually republished its embedded skills. Seeding declines when the
// running binary's version disagrees with APP_VERSION, and it skips versions
// that already exist, so "the skill did not update" is otherwise invisible
// without reading pod logs.
type SeedStatus struct {
	// Version is the version the embedded skills were published under. Empty
	// when seeding was declined.
	Version string `json:"version,omitempty"`
	// Declined explains why no seeding was attempted, if it was not.
	Declined string `json:"declined,omitempty"`
	// Skills reports the outcome per skill.
	Skills []seed.Result `json:"skills,omitempty"`
}

// statusResponse is the GET /status body.
type statusResponse struct {
	// BuildVersion is the release this binary was built from, or "dev".
	BuildVersion string     `json:"buildVersion"`
	Seed         SeedStatus `json:"seed"`
}

func handleStatus(buildVersion string, s SeedStatus) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(statusResponse{BuildVersion: buildVersion, Seed: s})
	}
}
