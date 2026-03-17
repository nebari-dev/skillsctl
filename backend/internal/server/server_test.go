package server_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"connectrpc.com/connect"

	"github.com/nebari-dev/skillctl/backend/internal/auth"
	"github.com/nebari-dev/skillctl/backend/internal/server"
	"github.com/nebari-dev/skillctl/backend/internal/store"
	sqlitestore "github.com/nebari-dev/skillctl/backend/internal/store/sqlite"
	"github.com/nebari-dev/skillctl/backend/internal/store/sqlite/migrations"
	skillctlv1 "github.com/nebari-dev/skillctl/gen/go/skillctl/v1"
	"github.com/nebari-dev/skillctl/gen/go/skillctl/v1/skillctlv1connect"
)

type stubValidator struct {
	claims *auth.Claims
	err    error
}

func (s *stubValidator) Validate(_ context.Context, _ string) (*auth.Claims, error) {
	return s.claims, s.err
}

func TestHealthz(t *testing.T) {
	srv := server.New(store.NewMemory(nil), nil, auth.Config{})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Errorf("expected body 'ok', got %q", string(body))
	}
}

func TestRPC_RequiresAuth(t *testing.T) {
	validator := &stubValidator{err: errors.New("invalid")}
	srv := server.New(store.NewMemory(nil), validator, auth.Config{})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := skillctlv1connect.NewRegistryServiceClient(http.DefaultClient, ts.URL)
	req := connect.NewRequest(&skillctlv1.ListSkillsRequest{})
	req.Header().Set("Authorization", "Bearer bad-token")
	_, err := client.ListSkills(context.Background(), req)
	if err == nil {
		t.Fatal("expected auth error, got nil")
	}
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeUnauthenticated {
		t.Errorf("code: got %v, want Unauthenticated", connectErr.Code())
	}
}

func TestRPC_NilValidator_PassesThrough(t *testing.T) {
	srv := server.New(store.NewMemory(nil), nil, auth.Config{})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := skillctlv1connect.NewRegistryServiceClient(http.DefaultClient, ts.URL)
	_, err := client.ListSkills(context.Background(), connect.NewRequest(&skillctlv1.ListSkillsRequest{}))
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

// TestIntegration_PublishAndRetrieve exercises the full server stack:
// HTTP -> interceptor (dev mode) -> service -> SQLite store -> response.
// This is the closest thing to a real server without starting a process.
func TestIntegration_PublishAndRetrieve(t *testing.T) {
	// Set up SQLite in-memory with migrations.
	db, err := sqlitestore.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := migrations.Run(context.Background(), db); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	repo := sqlitestore.New(db)
	srv := server.New(repo, nil, auth.Config{}) // nil validator = dev mode with dev claims
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	client := skillctlv1connect.NewRegistryServiceClient(http.DefaultClient, ts.URL)
	ctx := context.Background()

	// Step 1: Publish v1.0.0
	pubResp, err := client.PublishSkill(ctx, connect.NewRequest(&skillctlv1.PublishSkillRequest{
		Name:        "integration-test",
		Version:     "1.0.0",
		Description: "Integration test skill",
		Tags:        []string{"test", "go"},
		Changelog:   "Initial release",
		Content:     []byte("# Integration Test\n\nThis is the content."),
	}))
	if err != nil {
		t.Fatalf("PublishSkill v1.0.0: %v", err)
	}
	if pubResp.Msg.Skill.Name != "integration-test" {
		t.Errorf("skill name: got %q, want %q", pubResp.Msg.Skill.Name, "integration-test")
	}
	if pubResp.Msg.Skill.Owner != "dev-user" {
		t.Errorf("owner: got %q, want %q", pubResp.Msg.Skill.Owner, "dev-user")
	}
	if pubResp.Msg.Version.Digest == "" {
		t.Error("expected non-empty digest")
	}
	if !strings.HasPrefix(pubResp.Msg.Version.Digest, "sha256:") {
		t.Errorf("digest should start with sha256:, got %q", pubResp.Msg.Version.Digest)
	}
	v1Digest := pubResp.Msg.Version.Digest

	// Step 2: List skills - should contain the published skill.
	listResp, err := client.ListSkills(ctx, connect.NewRequest(&skillctlv1.ListSkillsRequest{}))
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}
	if len(listResp.Msg.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(listResp.Msg.Skills))
	}
	if listResp.Msg.Skills[0].Name != "integration-test" {
		t.Errorf("listed skill: got %q", listResp.Msg.Skills[0].Name)
	}

	// Step 3: Get skill details with versions.
	getResp, err := client.GetSkill(ctx, connect.NewRequest(&skillctlv1.GetSkillRequest{
		Name: "integration-test",
	}))
	if err != nil {
		t.Fatalf("GetSkill: %v", err)
	}
	if getResp.Msg.Skill.LatestVersion != "1.0.0" {
		t.Errorf("latest_version: got %q, want %q", getResp.Msg.Skill.LatestVersion, "1.0.0")
	}
	if len(getResp.Msg.Versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(getResp.Msg.Versions))
	}

	// Step 4: Get content by version.
	contentResp, err := client.GetSkillContent(ctx, connect.NewRequest(&skillctlv1.GetSkillContentRequest{
		Name:    "integration-test",
		Version: "1.0.0",
	}))
	if err != nil {
		t.Fatalf("GetSkillContent: %v", err)
	}
	if string(contentResp.Msg.Content) != "# Integration Test\n\nThis is the content." {
		t.Errorf("content: got %q", string(contentResp.Msg.Content))
	}

	// Step 5: Get content with matching digest.
	_, err = client.GetSkillContent(ctx, connect.NewRequest(&skillctlv1.GetSkillContentRequest{
		Name:    "integration-test",
		Version: "1.0.0",
		Digest:  v1Digest,
	}))
	if err != nil {
		t.Fatalf("GetSkillContent with matching digest: %v", err)
	}

	// Step 6: Get content with wrong digest - should fail.
	_, err = client.GetSkillContent(ctx, connect.NewRequest(&skillctlv1.GetSkillContentRequest{
		Name:    "integration-test",
		Version: "1.0.0",
		Digest:  "sha256:wrong",
	}))
	if err == nil {
		t.Fatal("expected digest mismatch error")
	}
	var connectErr *connect.Error
	if errors.As(err, &connectErr) && connectErr.Code() != connect.CodeFailedPrecondition {
		t.Errorf("expected FailedPrecondition, got %v", connectErr.Code())
	}

	// Step 7: Publish v2.0.0 - latest should advance.
	_, err = client.PublishSkill(ctx, connect.NewRequest(&skillctlv1.PublishSkillRequest{
		Name:        "integration-test",
		Version:     "2.0.0",
		Description: "Updated description",
		Tags:        []string{"test", "go", "v2"},
		Content:     []byte("# V2"),
	}))
	if err != nil {
		t.Fatalf("PublishSkill v2.0.0: %v", err)
	}

	getResp2, err := client.GetSkill(ctx, connect.NewRequest(&skillctlv1.GetSkillRequest{
		Name: "integration-test",
	}))
	if err != nil {
		t.Fatalf("GetSkill after v2: %v", err)
	}
	if getResp2.Msg.Skill.LatestVersion != "2.0.0" {
		t.Errorf("latest after v2: got %q", getResp2.Msg.Skill.LatestVersion)
	}
	if len(getResp2.Msg.Versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(getResp2.Msg.Versions))
	}

	// Step 8: Get latest content (empty version) - should return v2.
	latestResp, err := client.GetSkillContent(ctx, connect.NewRequest(&skillctlv1.GetSkillContentRequest{
		Name: "integration-test",
	}))
	if err != nil {
		t.Fatalf("GetSkillContent latest: %v", err)
	}
	if string(latestResp.Msg.Content) != "# V2" {
		t.Errorf("latest content: got %q, want %q", string(latestResp.Msg.Content), "# V2")
	}

	// Step 9: Duplicate version - should fail.
	_, err = client.PublishSkill(ctx, connect.NewRequest(&skillctlv1.PublishSkillRequest{
		Name:        "integration-test",
		Version:     "1.0.0",
		Description: "dup",
		Content:     []byte("dup"),
	}))
	if err == nil {
		t.Fatal("expected duplicate version error")
	}
	if errors.As(err, &connectErr) && connectErr.Code() != connect.CodeAlreadyExists {
		t.Errorf("expected AlreadyExists, got %v", connectErr.Code())
	}

	// Step 10: Nonexistent skill - should 404.
	_, err = client.GetSkillContent(ctx, connect.NewRequest(&skillctlv1.GetSkillContentRequest{
		Name:    "does-not-exist",
		Version: "1.0.0",
	}))
	if err == nil {
		t.Fatal("expected not found error")
	}
	if errors.As(err, &connectErr) && connectErr.Code() != connect.CodeNotFound {
		t.Errorf("expected NotFound, got %v", connectErr.Code())
	}
}

func TestAuthConfig_Enabled(t *testing.T) {
	cfg := auth.Config{
		IssuerURL: "https://keycloak.example.com/realms/test",
		ClientID:  "skillctl-cli",
	}
	srv := server.New(store.NewMemory(nil), nil, cfg)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/auth/config")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %q", ct)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"enabled":true`) {
		t.Errorf("expected enabled:true, got: %s", body)
	}
	if !strings.Contains(string(body), cfg.IssuerURL) {
		t.Errorf("expected issuer_url, got: %s", body)
	}
	if !strings.Contains(string(body), cfg.ClientID) {
		t.Errorf("expected client_id, got: %s", body)
	}
}

func TestAuthConfig_Disabled(t *testing.T) {
	srv := server.New(store.NewMemory(nil), nil, auth.Config{})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/auth/config")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"enabled":false`) {
		t.Errorf("expected enabled:false, got: %s", body)
	}
	// Should not contain issuer or client_id when disabled
	if strings.Contains(string(body), "issuer_url") {
		t.Errorf("expected no issuer_url when disabled, got: %s", body)
	}
}
