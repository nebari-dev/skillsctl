package server_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	"github.com/nebari-dev/skillctl/backend/internal/auth"
	"github.com/nebari-dev/skillctl/backend/internal/server"
	"github.com/nebari-dev/skillctl/backend/internal/store"
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
	srv := server.New(store.NewMemory(nil), nil)
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
	srv := server.New(store.NewMemory(nil), validator)
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
	srv := server.New(store.NewMemory(nil), nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := skillctlv1connect.NewRegistryServiceClient(http.DefaultClient, ts.URL)
	_, err := client.ListSkills(context.Background(), connect.NewRequest(&skillctlv1.ListSkillsRequest{}))
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}
