package server_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nebari-dev/skillsctl/backend/internal/auth"
	"github.com/nebari-dev/skillsctl/backend/internal/seed"
	"github.com/nebari-dev/skillsctl/backend/internal/server"
	"github.com/nebari-dev/skillsctl/backend/internal/store"
)

func TestHandleStatus(t *testing.T) {
	tests := []struct {
		name         string
		opts         []server.Option
		wantBuild    string
		wantVersion  string
		wantDeclined string
		wantSkills   []seed.Result
	}{
		{
			name:      "no option set reports dev and an empty seed",
			opts:      nil,
			wantBuild: "dev",
		},
		{
			name: "successful seed reports the version and outcome",
			opts: []server.Option{server.WithSeedStatus("0.2.3", server.SeedStatus{
				Version: "0.2.3",
				Skills:  []seed.Result{{Name: "skillsctl-usage", Version: "0.2.3", Outcome: seed.OutcomeSeeded}},
			})},
			wantBuild:   "0.2.3",
			wantVersion: "0.2.3",
			wantSkills:  []seed.Result{{Name: "skillsctl-usage", Version: "0.2.3", Outcome: seed.OutcomeSeeded}},
		},
		{
			name: "already-present seed is distinguishable from a fresh publish",
			opts: []server.Option{server.WithSeedStatus("0.2.3", server.SeedStatus{
				Version: "0.2.3",
				Skills:  []seed.Result{{Name: "skillsctl-usage", Version: "0.2.3", Outcome: seed.OutcomeAlreadyPresent}},
			})},
			wantBuild:   "0.2.3",
			wantVersion: "0.2.3",
			wantSkills:  []seed.Result{{Name: "skillsctl-usage", Version: "0.2.3", Outcome: seed.OutcomeAlreadyPresent}},
		},
		{
			name: "declined seed reports why and no version",
			opts: []server.Option{server.WithSeedStatus("0.2.0", server.SeedStatus{
				Declined: `APP_VERSION "0.2.1" does not match build version "0.2.0"`,
			})},
			wantBuild:    "0.2.0",
			wantDeclined: `APP_VERSION "0.2.1" does not match build version "0.2.0"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := server.New(store.NewMemory(nil), nil, auth.Config{}, tt.opts...)
			rec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/status", nil))

			if rec.Code != http.StatusOK {
				t.Fatalf("GET /status = %d, want %d", rec.Code, http.StatusOK)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}

			var got struct {
				BuildVersion string `json:"buildVersion"`
				Seed         struct {
					Version  string        `json:"version"`
					Declined string        `json:"declined"`
					Skills   []seed.Result `json:"skills"`
				} `json:"seed"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("unmarshal body %q: %v", rec.Body.String(), err)
			}

			if got.BuildVersion != tt.wantBuild {
				t.Errorf("buildVersion = %q, want %q", got.BuildVersion, tt.wantBuild)
			}
			if got.Seed.Version != tt.wantVersion {
				t.Errorf("seed.version = %q, want %q", got.Seed.Version, tt.wantVersion)
			}
			if got.Seed.Declined != tt.wantDeclined {
				t.Errorf("seed.declined = %q, want %q", got.Seed.Declined, tt.wantDeclined)
			}
			if len(got.Seed.Skills) != len(tt.wantSkills) {
				t.Fatalf("seed.skills = %v, want %v", got.Seed.Skills, tt.wantSkills)
			}
			for i, want := range tt.wantSkills {
				if got.Seed.Skills[i] != want {
					t.Errorf("seed.skills[%d] = %+v, want %+v", i, got.Seed.Skills[i], want)
				}
			}
		})
	}
}

// TestStatusNeedsNoAuth guards the endpoint staying reachable without a token,
// since its whole purpose is letting an operator check a deploy.
func TestStatusNeedsNoAuth(t *testing.T) {
	srv := server.New(store.NewMemory(nil), &stubValidator{err: errors.New("no token")}, auth.Config{IssuerURL: "https://example.test", ClientID: "x"})
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /status without a token = %d, want %d", rec.Code, http.StatusOK)
	}
}
