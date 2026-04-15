package server

import (
	"encoding/json"
	"net/http"

	"github.com/nebari-dev/skillsctl/internal/skillpkg"
)

func handleLimits(l skillpkg.Limits) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(l)
	}
}
