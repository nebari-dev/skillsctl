package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	skillsctlv1 "github.com/nebari-dev/skillsctl/gen/go/skillsctl/v1"

	"github.com/nebari-dev/skillsctl/cli/internal/api"
	"github.com/nebari-dev/skillsctl/cli/internal/testutil"
)

func TestClient_ListSkills(t *testing.T) {
	ts := testutil.NewStubServer(t, testutil.SeedSkills())
	client := api.NewClient(ts.URL)

	skills, err := client.ListSkills(context.Background(), nil, skillsctlv1.SkillSource_SKILL_SOURCE_UNSPECIFIED)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(skills))
	}
}

func TestClient_PublishSkill(t *testing.T) {
	ts := testutil.NewStubServer(t, nil)
	client := api.NewClient(ts.URL)

	skill, ver, err := client.PublishSkill(context.Background(),
		"test-skill", "1.0.0", "A test", "Initial", []string{"go"}, []byte("content"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skill.Name != "test-skill" {
		t.Errorf("expected name test-skill, got %q", skill.Name)
	}
	if ver.Digest == "" {
		t.Error("expected non-empty digest")
	}
}

func TestClient_WithToken_AttachesHeader(t *testing.T) {
	var gotAuth string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	client := api.NewClient(ts.URL, api.WithToken("test-token-123"))
	_, _ = client.ListSkills(context.Background(), nil, 0)

	if gotAuth != "Bearer test-token-123" {
		t.Errorf("expected 'Bearer test-token-123', got %q", gotAuth)
	}
}

func TestClient_WithoutToken_NoHeader(t *testing.T) {
	var gotAuth string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	client := api.NewClient(ts.URL)
	_, _ = client.ListSkills(context.Background(), nil, 0)

	if gotAuth != "" {
		t.Errorf("expected no auth header, got %q", gotAuth)
	}
}

func TestClient_GetSkill(t *testing.T) {
	ts := testutil.NewStubServer(t, testutil.SeedSkills())
	client := api.NewClient(ts.URL)

	tests := []struct {
		name    string
		skill   string
		wantErr bool
	}{
		{"existing", "data-pipeline", false},
		{"not found", "nope", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skill, _, err := client.GetSkill(context.Background(), tt.skill)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if skill.Name != tt.skill {
				t.Errorf("expected %q, got %q", tt.skill, skill.Name)
			}
		})
	}
}
