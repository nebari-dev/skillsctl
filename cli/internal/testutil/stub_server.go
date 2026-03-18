package testutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	skillsctlv1 "github.com/nebari-dev/skillsctl/gen/go/skillsctl/v1"
	"github.com/nebari-dev/skillsctl/gen/go/skillsctl/v1/skillsctlv1connect"
)

// StubRegistryService is a minimal ConnectRPC handler for CLI unit tests.
// It serves canned skill data without depending on any backend internals.
type StubRegistryService struct {
	skillsctlv1connect.UnimplementedRegistryServiceHandler
	Skills     []*skillsctlv1.Skill
	Content    map[string][]byte // keyed by skill name
	PublishErr error             // if set, PublishSkill returns this error
}

func (s *StubRegistryService) ListSkills(_ context.Context, _ *connect.Request[skillsctlv1.ListSkillsRequest]) (*connect.Response[skillsctlv1.ListSkillsResponse], error) {
	return connect.NewResponse(&skillsctlv1.ListSkillsResponse{Skills: s.Skills}), nil
}

func (s *StubRegistryService) GetSkill(_ context.Context, req *connect.Request[skillsctlv1.GetSkillRequest]) (*connect.Response[skillsctlv1.GetSkillResponse], error) {
	for _, sk := range s.Skills {
		if sk.Name == req.Msg.Name {
			return connect.NewResponse(&skillsctlv1.GetSkillResponse{Skill: sk}), nil
		}
	}
	return nil, connect.NewError(connect.CodeNotFound, nil)
}

func (s *StubRegistryService) PublishSkill(_ context.Context, req *connect.Request[skillsctlv1.PublishSkillRequest]) (*connect.Response[skillsctlv1.PublishSkillResponse], error) {
	if s.PublishErr != nil {
		return nil, s.PublishErr
	}
	skill := &skillsctlv1.Skill{
		Name:          req.Msg.Name,
		LatestVersion: req.Msg.Version,
		Owner:         "dev-user",
	}
	ver := &skillsctlv1.SkillVersion{
		Version: req.Msg.Version,
		Digest:  "sha256:stubdigest",
	}
	return connect.NewResponse(&skillsctlv1.PublishSkillResponse{
		Skill:   skill,
		Version: ver,
	}), nil
}

func (s *StubRegistryService) GetSkillContent(_ context.Context, req *connect.Request[skillsctlv1.GetSkillContentRequest]) (*connect.Response[skillsctlv1.GetSkillContentResponse], error) {
	if s.Content == nil {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	content, ok := s.Content[req.Msg.Name]
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	if req.Msg.Digest != "" && req.Msg.Digest != "sha256:gooddigest" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("digest mismatch"))
	}
	return connect.NewResponse(&skillsctlv1.GetSkillContentResponse{
		Content: content,
		Version: &skillsctlv1.SkillVersion{Version: "1.0.0", Digest: "sha256:gooddigest"},
	}), nil
}

// SeedSkills returns a standard set of test skills.
func SeedSkills() []*skillsctlv1.Skill {
	return []*skillsctlv1.Skill{
		{
			Name:          "data-pipeline",
			Description:   "Data pipeline utilities",
			Owner:         "data-eng",
			Tags:          []string{"data", "spark"},
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

// NewStubServer starts a test server running the StubRegistryService.
// Cleaned up automatically when the test finishes.
func NewStubServer(t *testing.T, skills []*skillsctlv1.Skill) *httptest.Server {
	return NewStubServerWithContent(t, skills, nil)
}

// NewStubServerWithContent starts a test server with skills and content.
func NewStubServerWithContent(t *testing.T, skills []*skillsctlv1.Skill, content map[string][]byte) *httptest.Server {
	t.Helper()
	stub := &StubRegistryService{Skills: skills, Content: content}
	mux := http.NewServeMux()
	path, handler := skillsctlv1connect.NewRegistryServiceHandler(stub)
	mux.Handle(path, handler)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

// NewStubServerFull starts a test server with skills, content, and an optional publish error.
func NewStubServerFull(t *testing.T, skills []*skillsctlv1.Skill, content map[string][]byte, publishErr error) *httptest.Server {
	t.Helper()
	stub := &StubRegistryService{Skills: skills, Content: content, PublishErr: publishErr}
	mux := http.NewServeMux()
	path, handler := skillsctlv1connect.NewRegistryServiceHandler(stub)
	mux.Handle(path, handler)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}
