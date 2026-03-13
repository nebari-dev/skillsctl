package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nebari-dev/skillctl/backend/internal/auth"
)

// TestAllowlistMiddleware verifies the middleware passes all requests through.
// All cases assert the same outcome because the middleware is currently a
// pass-through placeholder. When HTTP-level auth is added, these tests should
// be updated to assert differential behavior for allowlisted vs. non-allowlisted paths.
func TestAllowlistMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("reached"))
	})

	mw := auth.NewAllowlistMiddleware([]string{"/healthz"}, inner)

	tests := []struct {
		name     string
		path     string
		wantBody string
	}{
		{"allowlisted path", "/healthz", "reached"},
		{"non-allowlisted path", "/other", "reached"},
		{"partial match not allowlisted", "/healthz-other", "reached"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			mw.ServeHTTP(rec, req)
			if rec.Body.String() != tt.wantBody {
				t.Errorf("body: got %q, want %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}
