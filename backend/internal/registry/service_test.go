package registry_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	skillsctlv1 "github.com/nebari-dev/skillsctl/gen/go/skillsctl/v1"
	"github.com/nebari-dev/skillsctl/gen/go/skillsctl/v1/skillsctlv1connect"

	"github.com/nebari-dev/skillsctl/backend/internal/auth"
	"github.com/nebari-dev/skillsctl/backend/internal/registry"
	"github.com/nebari-dev/skillsctl/backend/internal/store"
)

func testSkills() []*skillsctlv1.Skill {
	return []*skillsctlv1.Skill{
		{
			Name:          "data-pipeline",
			Description:   "Data pipeline utilities",
			Owner:         "data-eng",
			Tags:          []string{"data", "spark", "go"},
			LatestVersion: "1.3.0",
			InstallCount:  47,
			Source:        skillsctlv1.SkillSource_SKILL_SOURCE_INTERNAL,
		},
		{
			Name:          "code-review",
			Description:   "Code review helpers",
			Owner:         "platform",
			Tags:          []string{"review", "quality"},
			LatestVersion: "0.9.1",
			InstallCount:  23,
			Source:        skillsctlv1.SkillSource_SKILL_SOURCE_INTERNAL,
		},
	}
}

func newTestClient(t *testing.T) skillsctlv1connect.RegistryServiceClient {
	t.Helper()
	svc := registry.NewService(store.NewMemory(testSkills()))
	mux := http.NewServeMux()
	path, handler := skillsctlv1connect.NewRegistryServiceHandler(svc)
	mux.Handle(path, handler)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return skillsctlv1connect.NewRegistryServiceClient(http.DefaultClient, ts.URL)
}

func TestRegistryService_ListSkills(t *testing.T) {
	tests := []struct {
		name      string
		req       *skillsctlv1.ListSkillsRequest
		wantCount int
	}{
		{
			name:      "list all",
			req:       &skillsctlv1.ListSkillsRequest{},
			wantCount: 2,
		},
		{
			name:      "filter by tag",
			req:       &skillsctlv1.ListSkillsRequest{Tags: []string{"go"}},
			wantCount: 1,
		},
		{
			name:      "filter by nonexistent tag",
			req:       &skillsctlv1.ListSkillsRequest{Tags: []string{"nope"}},
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
		req     *skillsctlv1.GetSkillRequest
		wantErr bool
	}{
		{
			name: "existing skill",
			req:  &skillsctlv1.GetSkillRequest{Name: "data-pipeline"},
		},
		{
			name:    "nonexistent skill",
			req:     &skillsctlv1.GetSkillRequest{Name: "nope"},
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

func TestRegistryService_PublishSkill(t *testing.T) {
	tests := []struct {
		name     string
		req      *skillsctlv1.PublishSkillRequest
		wantCode connect.Code
	}{
		{
			name: "valid publish",
			req: &skillsctlv1.PublishSkillRequest{
				Name:        "my-skill",
				Version:     "1.0.0",
				Description: "A useful skill",
				Tags:        []string{"go", "testing"},
				Content:     []byte("# My Skill"),
			},
		},
		{
			name: "invalid name",
			req: &skillsctlv1.PublishSkillRequest{
				Name:        "A",
				Version:     "1.0.0",
				Description: "desc",
				Content:     []byte("content"),
			},
			wantCode: connect.CodeInvalidArgument,
		},
		{
			name: "invalid version",
			req: &skillsctlv1.PublishSkillRequest{
				Name:        "my-skill",
				Version:     "not-semver",
				Description: "desc",
				Content:     []byte("content"),
			},
			wantCode: connect.CodeInvalidArgument,
		},
		{
			name: "empty content",
			req: &skillsctlv1.PublishSkillRequest{
				Name:        "my-skill",
				Version:     "1.0.0",
				Description: "desc",
				Content:     nil,
			},
			wantCode: connect.CodeInvalidArgument,
		},
		{
			name: "empty description",
			req: &skillsctlv1.PublishSkillRequest{
				Name:        "my-skill",
				Version:     "1.0.0",
				Description: "",
				Content:     []byte("content"),
			},
			wantCode: connect.CodeInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := registry.NewService(store.NewMemory(nil))
			ctx := auth.WithClaims(context.Background(), &auth.Claims{
				Subject: "user-123",
				Email:   "user@example.com",
			})
			resp, err := svc.PublishSkill(ctx, connect.NewRequest(tt.req))
			if tt.wantCode != 0 {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if connectErr := new(connect.Error); errors.As(err, &connectErr) {
					if connectErr.Code() != tt.wantCode {
						t.Errorf("expected code %v, got %v: %s", tt.wantCode, connectErr.Code(), connectErr.Message())
					}
				} else {
					t.Errorf("expected connect.Error, got %T: %v", err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.Msg.Skill.Name != tt.req.Name {
				t.Errorf("expected skill name %q, got %q", tt.req.Name, resp.Msg.Skill.Name)
			}
			if resp.Msg.Version.Version != tt.req.Version {
				t.Errorf("expected version %q, got %q", tt.req.Version, resp.Msg.Version.Version)
			}
			if resp.Msg.Version.Digest == "" {
				t.Error("expected non-empty digest")
			}
		})
	}
}

func TestRegistryService_PublishSkill_Unauthenticated(t *testing.T) {
	svc := registry.NewService(store.NewMemory(nil))
	_, err := svc.PublishSkill(context.Background(), connect.NewRequest(&skillsctlv1.PublishSkillRequest{
		Name:        "my-skill",
		Version:     "1.0.0",
		Description: "desc",
		Content:     []byte("content"),
	}))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		if connectErr.Code() != connect.CodeUnauthenticated {
			t.Errorf("expected CodeUnauthenticated, got %v", connectErr.Code())
		}
	} else {
		t.Errorf("expected connect.Error, got %T: %v", err, err)
	}
}

func TestRegistryService_GetSkillContent(t *testing.T) {
	svc := registry.NewService(store.NewMemory(nil))
	ctx := auth.WithClaims(context.Background(), &auth.Claims{
		Subject: "user-123",
		Email:   "user@example.com",
	})
	_, err := svc.PublishSkill(ctx, connect.NewRequest(&skillsctlv1.PublishSkillRequest{
		Name:        "my-skill",
		Version:     "1.0.0",
		Description: "A useful skill",
		Content:     []byte("# My Skill\nContent here"),
	}))
	if err != nil {
		t.Fatalf("setup publish: %v", err)
	}

	tests := []struct {
		name     string
		req      *skillsctlv1.GetSkillContentRequest
		wantCode connect.Code
	}{
		{
			name: "get by name and version",
			req:  &skillsctlv1.GetSkillContentRequest{Name: "my-skill", Version: "1.0.0"},
		},
		{
			name: "get latest",
			req:  &skillsctlv1.GetSkillContentRequest{Name: "my-skill"},
		},
		{
			name:     "nonexistent skill",
			req:      &skillsctlv1.GetSkillContentRequest{Name: "nope", Version: "1.0.0"},
			wantCode: connect.CodeNotFound,
		},
		{
			name:     "digest mismatch",
			req:      &skillsctlv1.GetSkillContentRequest{Name: "my-skill", Version: "1.0.0", Digest: "sha256:wrong"},
			wantCode: connect.CodeFailedPrecondition,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := svc.GetSkillContent(ctx, connect.NewRequest(tt.req))
			if tt.wantCode != 0 {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if connectErr := new(connect.Error); errors.As(err, &connectErr) {
					if connectErr.Code() != tt.wantCode {
						t.Errorf("expected code %v, got %v", tt.wantCode, connectErr.Code())
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(resp.Msg.Content) == 0 {
				t.Error("expected non-empty content")
			}
			if resp.Msg.Version == nil {
				t.Error("expected version metadata")
			}
		})
	}
}

func TestRegistryService_GetSkillContent_Unauthenticated(t *testing.T) {
	svc := registry.NewService(store.NewMemory(nil))
	ctx := auth.WithClaims(context.Background(), &auth.Claims{
		Subject: "user-123",
		Email:   "user@example.com",
	})
	_, err := svc.PublishSkill(ctx, connect.NewRequest(&skillsctlv1.PublishSkillRequest{
		Name:        "my-skill",
		Version:     "1.0.0",
		Description: "A useful skill",
		Content:     []byte("# My Skill"),
	}))
	if err != nil {
		t.Fatalf("setup publish: %v", err)
	}

	// GetSkillContent without claims should succeed (read operation)
	resp, err := svc.GetSkillContent(context.Background(), connect.NewRequest(&skillsctlv1.GetSkillContentRequest{
		Name:    "my-skill",
		Version: "1.0.0",
	}))
	if err != nil {
		t.Fatalf("expected unauthenticated GetSkillContent to succeed, got: %v", err)
	}
	if len(resp.Msg.Content) == 0 {
		t.Error("expected non-empty content")
	}
}
