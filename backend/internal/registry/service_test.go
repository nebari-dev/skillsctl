package registry_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	skillctlv1 "github.com/nebari-dev/skillctl/gen/go/skillctl/v1"
	"github.com/nebari-dev/skillctl/gen/go/skillctl/v1/skillctlv1connect"

	"github.com/nebari-dev/skillctl/backend/internal/registry"
	"github.com/nebari-dev/skillctl/backend/internal/store"
)

func testSkills() []*skillctlv1.Skill {
	return []*skillctlv1.Skill{
		{
			Name:          "data-pipeline",
			Description:   "Data pipeline utilities",
			Owner:         "data-eng",
			Tags:          []string{"data", "spark", "go"},
			LatestVersion: "1.3.0",
			InstallCount:  47,
			Source:        skillctlv1.SkillSource_SKILL_SOURCE_INTERNAL,
		},
		{
			Name:          "code-review",
			Description:   "Code review helpers",
			Owner:         "platform",
			Tags:          []string{"review", "quality"},
			LatestVersion: "0.9.1",
			InstallCount:  23,
			Source:        skillctlv1.SkillSource_SKILL_SOURCE_INTERNAL,
		},
	}
}

func newTestClient(t *testing.T) skillctlv1connect.RegistryServiceClient {
	t.Helper()
	svc := registry.NewService(store.NewMemory(testSkills()))
	mux := http.NewServeMux()
	path, handler := skillctlv1connect.NewRegistryServiceHandler(svc)
	mux.Handle(path, handler)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return skillctlv1connect.NewRegistryServiceClient(http.DefaultClient, ts.URL)
}

func TestRegistryService_ListSkills(t *testing.T) {
	tests := []struct {
		name      string
		req       *skillctlv1.ListSkillsRequest
		wantCount int
	}{
		{
			name:      "list all",
			req:       &skillctlv1.ListSkillsRequest{},
			wantCount: 2,
		},
		{
			name:      "filter by tag",
			req:       &skillctlv1.ListSkillsRequest{Tags: []string{"go"}},
			wantCount: 1,
		},
		{
			name:      "filter by nonexistent tag",
			req:       &skillctlv1.ListSkillsRequest{Tags: []string{"nope"}},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t)
			resp, err := client.ListSkills(context.Background(), connect.NewRequest(tt.req))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(resp.Msg.Skills) != tt.wantCount {
				t.Errorf("expected %d skills, got %d", tt.wantCount, len(resp.Msg.Skills))
			}
		})
	}
}

func TestRegistryService_GetSkill(t *testing.T) {
	tests := []struct {
		name    string
		req     *skillctlv1.GetSkillRequest
		wantErr bool
	}{
		{
			name: "existing skill",
			req:  &skillctlv1.GetSkillRequest{Name: "data-pipeline"},
		},
		{
			name:    "nonexistent skill",
			req:     &skillctlv1.GetSkillRequest{Name: "nope"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t)
			resp, err := client.GetSkill(context.Background(), connect.NewRequest(tt.req))
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.Msg.Skill.Name != tt.req.Name {
				t.Errorf("expected %q, got %q", tt.req.Name, resp.Msg.Skill.Name)
			}
		})
	}
}
