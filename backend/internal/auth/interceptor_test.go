package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	skillctlv1 "github.com/nebari-dev/skillctl/gen/go/skillctl/v1"
	"github.com/nebari-dev/skillctl/gen/go/skillctl/v1/skillctlv1connect"

	"github.com/nebari-dev/skillctl/backend/internal/auth"
	"github.com/nebari-dev/skillctl/backend/internal/registry"
	"github.com/nebari-dev/skillctl/backend/internal/store"
)

type stubValidator struct {
	claims *auth.Claims
	err    error
}

func (s *stubValidator) Validate(_ context.Context, _ string) (*auth.Claims, error) {
	return s.claims, s.err
}

func newTestServer(t *testing.T, v auth.TokenValidator) (*httptest.Server, skillctlv1connect.RegistryServiceClient) {
	t.Helper()
	mux := http.NewServeMux()
	interceptor := auth.NewInterceptor(v)
	path, handler := skillctlv1connect.NewRegistryServiceHandler(
		registry.NewService(store.NewMemory(nil)),
		connect.WithInterceptors(interceptor),
	)
	mux.Handle(path, handler)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	client := skillctlv1connect.NewRegistryServiceClient(http.DefaultClient, ts.URL)
	return ts, client
}

func TestInterceptor(t *testing.T) {
	validClaims := &auth.Claims{Subject: "user-1", Email: "u@example.com"}

	tests := []struct {
		name        string
		validator   auth.TokenValidator
		authHeader  string
		wantSuccess bool
		wantCode    connect.Code
	}{
		{
			name:        "valid token",
			validator:   &stubValidator{claims: validClaims},
			authHeader:  "Bearer valid-token",
			wantSuccess: true,
		},
		{
			name:       "missing header",
			validator:  &stubValidator{claims: validClaims},
			authHeader: "",
			wantCode:   connect.CodeUnauthenticated,
		},
		{
			name:       "malformed prefix",
			validator:  &stubValidator{claims: validClaims},
			authHeader: "Basic xyz",
			wantCode:   connect.CodeUnauthenticated,
		},
		{
			name:       "empty bearer token",
			validator:  &stubValidator{claims: validClaims},
			authHeader: "Bearer ",
			wantCode:   connect.CodeUnauthenticated,
		},
		{
			name:       "validator error",
			validator:  &stubValidator{err: errors.New("bad token")},
			authHeader: "Bearer bad-token",
			wantCode:   connect.CodeUnauthenticated,
		},
		{
			name:        "nil validator injects dev claims",
			validator:   nil,
			authHeader:  "",
			wantSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, client := newTestServer(t, tt.validator)
			req := connect.NewRequest(&skillctlv1.ListSkillsRequest{})
			if tt.authHeader != "" {
				req.Header().Set("Authorization", tt.authHeader)
			}
			_, err := client.ListSkills(context.Background(), req)
			if tt.wantSuccess {
				if err != nil {
					t.Fatalf("expected success, got error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			var connectErr *connect.Error
			if !errors.As(err, &connectErr) {
				t.Fatalf("expected connect.Error, got %T: %v", err, err)
			}
			if connectErr.Code() != tt.wantCode {
				t.Errorf("code: got %v, want %v", connectErr.Code(), tt.wantCode)
			}
		})
	}
}

// TestInterceptor_DevModeClaimsInContext verifies that nil validator
// injects dev claims so write operations work in local development.
func TestInterceptor_DevModeClaimsInContext(t *testing.T) {
	var gotClaims *auth.Claims
	interceptor := auth.NewInterceptor(nil)
	capturer := connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			c, ok := auth.ClaimsFromContext(ctx)
			if ok {
				gotClaims = c
			}
			return next(ctx, req)
		}
	})

	mux := http.NewServeMux()
	path, handler := skillctlv1connect.NewRegistryServiceHandler(
		registry.NewService(store.NewMemory(nil)),
		connect.WithInterceptors(interceptor, capturer),
	)
	mux.Handle(path, handler)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	client := skillctlv1connect.NewRegistryServiceClient(http.DefaultClient, ts.URL)
	_, err := client.ListSkills(context.Background(), connect.NewRequest(&skillctlv1.ListSkillsRequest{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotClaims == nil {
		t.Fatal("expected dev claims in context, got nil")
	}
	if gotClaims.Subject != "dev-user" {
		t.Errorf("subject: got %q, want %q", gotClaims.Subject, "dev-user")
	}
	if gotClaims.Email != "dev@localhost" {
		t.Errorf("email: got %q, want %q", gotClaims.Email, "dev@localhost")
	}
}

// TestInterceptor_ClaimsInContext verifies that the interceptor injects
// claims into context so handlers can access them via ClaimsFromContext.
func TestInterceptor_ClaimsInContext(t *testing.T) {
	wantClaims := &auth.Claims{Subject: "user-1", Email: "u@example.com", Groups: []string{"devs"}}
	validator := &stubValidator{claims: wantClaims}

	var gotClaims *auth.Claims
	interceptor := auth.NewInterceptor(validator)
	capturer := connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			c, ok := auth.ClaimsFromContext(ctx)
			if ok {
				gotClaims = c
			}
			return next(ctx, req)
		}
	})

	mux := http.NewServeMux()
	path, handler := skillctlv1connect.NewRegistryServiceHandler(
		registry.NewService(store.NewMemory(nil)),
		connect.WithInterceptors(interceptor, capturer),
	)
	mux.Handle(path, handler)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	client := skillctlv1connect.NewRegistryServiceClient(http.DefaultClient, ts.URL)
	req := connect.NewRequest(&skillctlv1.ListSkillsRequest{})
	req.Header().Set("Authorization", "Bearer valid-token")
	_, err := client.ListSkills(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotClaims == nil {
		t.Fatal("expected claims in context, got nil")
	}
	if gotClaims.Subject != wantClaims.Subject {
		t.Errorf("subject: got %q, want %q", gotClaims.Subject, wantClaims.Subject)
	}
	if gotClaims.Email != wantClaims.Email {
		t.Errorf("email: got %q, want %q", gotClaims.Email, wantClaims.Email)
	}
}
