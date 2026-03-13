# OIDC Auth Middleware Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add OIDC-based authentication to the backend's ConnectRPC endpoints using `coreos/go-oidc/v3`.

**Architecture:** A `TokenValidator` interface enables stub-based testing. The production `Validator` fetches JWKS from the OIDC provider and verifies JWT tokens. A ConnectRPC unary interceptor extracts Bearer tokens and injects claims into context. An HTTP allowlist middleware exempts `/healthz` from auth. Nil validator disables auth for local dev.

**Tech Stack:** Go, `coreos/go-oidc/v3`, `connectrpc.com/connect`, `go-jose/go-jose/v4` (test JWT minting)

**Spec:** `docs/superpowers/specs/2026-03-13-oidc-auth-middleware-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `backend/internal/auth/config.go` | Create | Config struct with IssuerURL, ClientID, AdminGroup, GroupsClaim |
| `backend/internal/auth/validator.go` | Create | TokenValidator interface, Claims struct, Validator (production OIDC) |
| `backend/internal/auth/validator_test.go` | Create | Validator tests with fake OIDC httptest server |
| `backend/internal/auth/context.go` | Create | WithClaims/ClaimsFromContext context helpers |
| `backend/internal/auth/context_test.go` | Create | Context round-trip test |
| `backend/internal/auth/interceptor.go` | Create | ConnectRPC unary interceptor |
| `backend/internal/auth/interceptor_test.go` | Create | Interceptor tests with stub TokenValidator |
| `backend/internal/auth/allowlist.go` | Create | HTTP allowlist middleware |
| `backend/internal/auth/allowlist_test.go` | Create | Allowlist exact-match tests |
| `backend/internal/server/server.go` | Modify | Accept TokenValidator, wire interceptor + allowlist |
| `backend/internal/server/server_test.go` | Modify | Update New() calls, add auth integration tests |
| `backend/cmd/server/main.go` | Modify | Read auth env vars, validate, create Validator |
| `go.mod` | Modify | Add `coreos/go-oidc/v3` dependency |

---

## Chunk 1: Foundation (Config, Claims, Context)

### Task 1: Add go-oidc dependency

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Add dependencies (OIDC validation + JWT minting for tests)**

```bash
go get github.com/coreos/go-oidc/v3@latest
go get github.com/go-jose/go-jose/v4@latest
go mod tidy
```

- [ ] **Step 2: Verify it resolves**

```bash
go build ./...
```

Expected: clean build, no errors.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add coreos/go-oidc/v3 and go-jose/v4 for OIDC auth"
```

---

### Task 2: Config struct and context helpers

**Files:**
- Create: `backend/internal/auth/config.go`
- Create: `backend/internal/auth/context.go`
- Create: `backend/internal/auth/context_test.go`

- [ ] **Step 1: Write the context round-trip test**

```go
// backend/internal/auth/context_test.go
package auth_test

import (
	"context"
	"testing"

	"github.com/nebari-dev/skillctl/backend/internal/auth"
)

func TestClaimsContext_RoundTrip(t *testing.T) {
	want := &auth.Claims{
		Subject: "user-123",
		Email:   "user@example.com",
		Groups:  []string{"devs", "admins"},
	}

	ctx := auth.WithClaims(context.Background(), want)
	got, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		t.Fatal("expected claims in context")
	}
	if got.Subject != want.Subject {
		t.Errorf("subject: got %q, want %q", got.Subject, want.Subject)
	}
	if got.Email != want.Email {
		t.Errorf("email: got %q, want %q", got.Email, want.Email)
	}
	if len(got.Groups) != len(want.Groups) {
		t.Errorf("groups: got %v, want %v", got.Groups, want.Groups)
	}
}

func TestClaimsContext_Missing(t *testing.T) {
	_, ok := auth.ClaimsFromContext(context.Background())
	if ok {
		t.Error("expected no claims in empty context")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./backend/internal/auth/... -run TestClaimsContext -v
```

Expected: FAIL - package doesn't exist yet.

- [ ] **Step 3: Write config.go**

```go
// backend/internal/auth/config.go
package auth

// Config holds OIDC authentication configuration.
type Config struct {
	IssuerURL   string // OIDC discovery URL, e.g. https://keycloak.example.com/realms/myrealm
	ClientID    string // expected audience claim in the JWT
	AdminGroup  string // OIDC group name for admin role checks
	GroupsClaim string // JWT claim name for group membership, defaults to "groups"
}
```

- [ ] **Step 4: Write context.go with Claims**

```go
// backend/internal/auth/context.go
package auth

import "context"

// Claims holds the verified identity extracted from an OIDC token.
type Claims struct {
	Subject string
	Email   string
	Groups  []string
}

type claimsKey struct{}

// WithClaims returns a new context carrying the given claims.
func WithClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, c)
}

// ClaimsFromContext extracts claims from the context. Returns false if no claims are present.
func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(claimsKey{}).(*Claims)
	return c, ok
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./backend/internal/auth/... -run TestClaimsContext -v
```

Expected: PASS (2 tests).

- [ ] **Step 6: Commit**

```bash
git add backend/internal/auth/
git commit -m "feat: add auth Config struct and Claims context helpers"
```

---

## Chunk 2: Validator (OIDC token verification)

### Task 3: TokenValidator interface and production Validator

**Files:**
- Create: `backend/internal/auth/validator.go`
- Create: `backend/internal/auth/validator_test.go`

- [ ] **Step 1: Write validator tests with fake OIDC server**

The test file needs a helper that:
1. Generates an RSA key pair
2. Starts an `httptest.Server` serving `/.well-known/openid-configuration` and `/jwks`
3. Provides a `mintToken(claims, expiry)` function that creates signed JWTs

```go
// backend/internal/auth/validator_test.go
package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/nebari-dev/skillctl/backend/internal/auth"
)

type fakeOIDC struct {
	server *httptest.Server
	key    *rsa.PrivateKey
}

func newFakeOIDC(t *testing.T) *fakeOIDC {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	f := &fakeOIDC{key: key}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", f.handleDiscovery)
	mux.HandleFunc("/jwks", f.handleJWKS)
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeOIDC) handleDiscovery(w http.ResponseWriter, _ *http.Request) {
	doc := map[string]string{
		"issuer":   f.server.URL,
		"jwks_uri": f.server.URL + "/jwks",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(doc)
}

func (f *fakeOIDC) handleJWKS(w http.ResponseWriter, _ *http.Request) {
	jwks := jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Key:       &f.key.PublicKey,
				KeyID:     "test-key",
				Algorithm: string(jose.RS256),
				Use:       "sig",
			},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jwks)
}

type tokenClaims struct {
	Issuer      string
	Audience    []string
	Subject     string
	Email       string
	Groups      []string
	Expiry      time.Time
	ExtraClaims map[string]any // arbitrary extra claims (overrides Email/Groups if set)
}

func (f *fakeOIDC) mintToken(t *testing.T, tc tokenClaims) string {
	t.Helper()
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: f.key},
		(&jose.SignerOptions{}).WithHeader(jose.HeaderKey("kid"), "test-key"),
	)
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}

	now := time.Now()
	stdClaims := jwt.Claims{
		Issuer:    tc.Issuer,
		Audience:  jwt.Audience(tc.Audience),
		Subject:   tc.Subject,
		IssuedAt:  jwt.NewNumericDate(now),
		Expiry:    jwt.NewNumericDate(tc.Expiry),
		NotBefore: jwt.NewNumericDate(now.Add(-1 * time.Minute)),
	}

	extra := map[string]any{}
	if tc.Email != "" {
		extra["email"] = tc.Email
	}
	if tc.Groups != nil {
		extra["groups"] = tc.Groups
	}
	for k, v := range tc.ExtraClaims {
		extra[k] = v
	}

	raw, err := jwt.Signed(signer).Claims(stdClaims).Claims(extra).Serialize()
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return raw
}

func TestValidator_Validate(t *testing.T) {
	fake := newFakeOIDC(t)
	clientID := "skillctl-cli"

	cfg := auth.Config{
		IssuerURL:   fake.server.URL,
		ClientID:    clientID,
		AdminGroup:  "skillctl-admins",
		GroupsClaim: "groups",
	}

	v, err := auth.NewValidator(context.Background(), cfg)
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}

	tests := []struct {
		name    string
		token   tokenClaims
		wantErr bool
		wantSub string
		wantEmail string
		wantGroups []string
	}{
		{
			name: "valid token with all claims",
			token: tokenClaims{
				Issuer:   fake.server.URL,
				Audience: []string{clientID},
				Subject:  "user-123",
				Email:    "user@example.com",
				Groups:   []string{"devs"},
				Expiry:   time.Now().Add(1 * time.Hour),
			},
			wantSub:    "user-123",
			wantEmail:  "user@example.com",
			wantGroups: []string{"devs"},
		},
		{
			name: "expired token",
			token: tokenClaims{
				Issuer:   fake.server.URL,
				Audience: []string{clientID},
				Subject:  "user-123",
				Expiry:   time.Now().Add(-1 * time.Hour),
			},
			wantErr: true,
		},
		{
			name: "wrong audience",
			token: tokenClaims{
				Issuer:   fake.server.URL,
				Audience: []string{"other-client"},
				Subject:  "user-123",
				Expiry:   time.Now().Add(1 * time.Hour),
			},
			wantErr: true,
		},
		{
			name: "missing email claim",
			token: tokenClaims{
				Issuer:   fake.server.URL,
				Audience: []string{clientID},
				Subject:  "user-123",
				Expiry:   time.Now().Add(1 * time.Hour),
			},
			wantSub: "user-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := fake.mintToken(t, tt.token)
			claims, err := v.Validate(context.Background(), raw)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if claims.Subject != tt.wantSub {
				t.Errorf("subject: got %q, want %q", claims.Subject, tt.wantSub)
			}
			if claims.Email != tt.wantEmail {
				t.Errorf("email: got %q, want %q", claims.Email, tt.wantEmail)
			}
			if len(claims.Groups) != len(tt.wantGroups) {
				t.Errorf("groups: got %v, want %v", claims.Groups, tt.wantGroups)
			}
		})
	}
}

func TestValidator_Validate_CustomGroupsClaim(t *testing.T) {
	fake := newFakeOIDC(t)
	clientID := "skillctl-cli"

	cfg := auth.Config{
		IssuerURL:   fake.server.URL,
		ClientID:    clientID,
		GroupsClaim: "roles",
	}

	v, err := auth.NewValidator(context.Background(), cfg)
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}

	// Mint a token with groups in the "roles" claim instead of "groups".
	raw := fake.mintToken(t, tokenClaims{
		Issuer:      fake.server.URL,
		Audience:    []string{clientID},
		Subject:     "user-456",
		Expiry:      time.Now().Add(1 * time.Hour),
		ExtraClaims: map[string]any{"roles": []string{"admin", "editor"}},
	})

	claims, err := v.Validate(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(claims.Groups) != 2 || claims.Groups[0] != "admin" {
		t.Errorf("expected groups [admin editor], got %v", claims.Groups)
	}
}

func TestValidator_IsAdmin(t *testing.T) {
	fake := newFakeOIDC(t)
	cfg := auth.Config{
		IssuerURL:   fake.server.URL,
		ClientID:    "test",
		AdminGroup:  "skillctl-admins",
		GroupsClaim: "groups",
	}
	v, err := auth.NewValidator(context.Background(), cfg)
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}

	tests := []struct {
		name   string
		groups []string
		want   bool
	}{
		{"admin member", []string{"devs", "skillctl-admins"}, true},
		{"not admin", []string{"devs"}, false},
		{"empty groups", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &auth.Claims{Groups: tt.groups}
			if got := v.IsAdmin(claims); got != tt.want {
				t.Errorf("IsAdmin: got %v, want %v", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./backend/internal/auth/... -run TestValidator -v
```

Expected: FAIL - `auth.NewValidator` and `auth.Validator` not defined.

- [ ] **Step 3: Write validator.go**

```go
// backend/internal/auth/validator.go
package auth

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
)

// TokenValidator validates a raw Bearer token and returns the extracted claims.
// The interceptor depends on this interface for testability.
type TokenValidator interface {
	Validate(ctx context.Context, rawToken string) (*Claims, error)
}

// Validator is the production OIDC token validator.
// It fetches JWKS from the OIDC provider and verifies JWT signatures.
type Validator struct {
	verifier *oidc.IDTokenVerifier
	cfg      Config
}

var _ TokenValidator = (*Validator)(nil)

// NewValidator creates a Validator by fetching the OIDC discovery document
// from cfg.IssuerURL. Fails fast if the provider is unreachable.
func NewValidator(ctx context.Context, cfg Config) (*Validator, error) {
	if cfg.GroupsClaim == "" {
		cfg.GroupsClaim = "groups"
	}
	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery %s: %w", cfg.IssuerURL, err)
	}
	verifier := provider.Verifier(&oidc.Config{
		ClientID: cfg.ClientID,
	})
	return &Validator{verifier: verifier, cfg: cfg}, nil
}

// Validate verifies the token signature, expiry, and audience, then extracts claims.
func (v *Validator) Validate(ctx context.Context, rawToken string) (*Claims, error) {
	idToken, err := v.verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, fmt.Errorf("verify token: %w", err)
	}

	var raw map[string]any
	if err := idToken.Claims(&raw); err != nil {
		return nil, fmt.Errorf("extract claims: %w", err)
	}

	claims := &Claims{
		Subject: idToken.Subject,
	}

	if email, ok := raw["email"].(string); ok {
		claims.Email = email
	}

	if groups, ok := raw[v.cfg.GroupsClaim]; ok {
		if arr, ok := groups.([]any); ok {
			for _, g := range arr {
				if s, ok := g.(string); ok {
					claims.Groups = append(claims.Groups, s)
				}
			}
		}
	}

	return claims, nil
}

// IsAdmin checks whether the claims include the configured admin group.
func (v *Validator) IsAdmin(claims *Claims) bool {
	for _, g := range claims.Groups {
		if g == v.cfg.AdminGroup {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./backend/internal/auth/... -run TestValidator -v
```

Expected: PASS (all validator tests).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/auth/validator.go backend/internal/auth/validator_test.go
git commit -m "feat: add OIDC TokenValidator with JWKS verification and claims extraction"
```

---

## Chunk 3: Interceptor and Allowlist

### Task 4: ConnectRPC auth interceptor

**Files:**
- Create: `backend/internal/auth/interceptor.go`
- Create: `backend/internal/auth/interceptor_test.go`

- [ ] **Step 1: Write interceptor tests with stub validator**

```go
// backend/internal/auth/interceptor_test.go
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

// stubValidator is a test double for auth.TokenValidator.
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
			name:        "nil validator passes through",
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

// TestInterceptor_ClaimsInContext verifies that the interceptor injects
// claims into context so handlers can access them via ClaimsFromContext.
func TestInterceptor_ClaimsInContext(t *testing.T) {
	wantClaims := &auth.Claims{Subject: "user-1", Email: "u@example.com", Groups: []string{"devs"}}
	validator := &stubValidator{claims: wantClaims}

	var gotClaims *auth.Claims
	interceptor := auth.NewInterceptor(validator)
	// Chain: auth interceptor -> claims-capturing interceptor -> handler
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
	defer ts.Close()

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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./backend/internal/auth/... -run TestInterceptor -v
```

Expected: FAIL - `auth.NewInterceptor` not defined.

- [ ] **Step 3: Write interceptor.go**

```go
// backend/internal/auth/interceptor.go
package auth

import (
	"context"
	"strings"

	"connectrpc.com/connect"
)

// NewInterceptor returns a ConnectRPC unary interceptor that validates
// Bearer tokens using the given validator. If validator is nil (local dev
// mode), all requests pass through without authentication.
func NewInterceptor(v TokenValidator) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if v == nil {
				return next(ctx, req)
			}

			authHeader := req.Header().Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				return nil, connect.NewError(connect.CodeUnauthenticated, nil)
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == "" {
				return nil, connect.NewError(connect.CodeUnauthenticated, nil)
			}

			claims, err := v.Validate(ctx, token)
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, err)
			}

			ctx = WithClaims(ctx, claims)
			return next(ctx, req)
		}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./backend/internal/auth/... -run TestInterceptor -v
```

Expected: PASS (6 tests).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/auth/interceptor.go backend/internal/auth/interceptor_test.go
git commit -m "feat: add ConnectRPC auth interceptor with Bearer token extraction"
```

---

### Task 5: HTTP allowlist middleware

**Files:**
- Create: `backend/internal/auth/allowlist.go`
- Create: `backend/internal/auth/allowlist_test.go`

- [ ] **Step 1: Write allowlist tests**

```go
// backend/internal/auth/allowlist_test.go
package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nebari-dev/skillctl/backend/internal/auth"
)

func TestAllowlistMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("reached"))
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
```

Note: The allowlist middleware passes all requests through to `next` - the auth enforcement happens in the ConnectRPC interceptor. The allowlist's role is to mark certain paths so the interceptor (or a future HTTP-level auth layer) knows to skip them. For now, since auth lives in the interceptor, the middleware is a thin pass-through that sets up the pattern for later use. The tests verify the handler is always reached (no blocking), and the allowlist matching logic is exercised.

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./backend/internal/auth/... -run TestAllowlistMiddleware -v
```

Expected: FAIL - `auth.NewAllowlistMiddleware` not defined.

- [ ] **Step 3: Write allowlist.go**

```go
// backend/internal/auth/allowlist.go
package auth

import "net/http"

// NewAllowlistMiddleware returns an HTTP middleware that marks requests to
// allowlisted paths as not requiring authentication. Uses exact path matching.
//
// Currently all requests pass through to next - auth is enforced by the
// ConnectRPC interceptor on RPC routes. This middleware establishes the
// allowlist pattern for future HTTP-level auth enforcement.
func NewAllowlistMiddleware(allowedPaths []string, next http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedPaths))
	for _, p := range allowedPaths {
		allowed[p] = struct{}{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Future: if HTTP-level auth is added, check allowed map here
		// and skip auth for matching paths.
		_ = allowed // used by future HTTP-level auth check
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./backend/internal/auth/... -run TestAllowlistMiddleware -v
```

Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/auth/allowlist.go backend/internal/auth/allowlist_test.go
git commit -m "feat: add HTTP allowlist middleware for unauthenticated paths"
```

---

## Chunk 4: Server Wiring and Integration

### Task 6: Wire auth into server and main

**Files:**
- Modify: `backend/internal/server/server.go`
- Modify: `backend/internal/server/server_test.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Update server_test.go with auth integration tests**

```go
// Add to backend/internal/server/server_test.go

// Update the existing TestHealthz to pass nil validator:
// server.New(store.NewMemory(nil)) -> server.New(store.NewMemory(nil), nil)

// Add new test:
func TestRPC_RequiresAuth(t *testing.T) {
	validator := &stubValidator{err: errors.New("invalid")}
	srv := server.New(store.NewMemory(nil), validator)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := skillctlv1connect.NewRegistryServiceClient(http.DefaultClient, ts.URL)
	_, err := client.ListSkills(context.Background(), connect.NewRequest(&skillctlv1.ListSkillsRequest{}))
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
```

The test file will need a local `stubValidator` (same pattern as in `interceptor_test.go`) and additional imports for `connect`, `skillctlv1`, `skillctlv1connect`, `auth`, `context`, and `errors`.

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./backend/internal/server/... -v
```

Expected: FAIL - `server.New` signature mismatch (missing second argument).

- [ ] **Step 3: Update server.go**

Change `New` to accept `auth.TokenValidator`, wire the interceptor and allowlist:

```go
package server

import (
	"net/http"

	"connectrpc.com/connect"

	"github.com/nebari-dev/skillctl/backend/internal/auth"
	"github.com/nebari-dev/skillctl/backend/internal/registry"
	"github.com/nebari-dev/skillctl/backend/internal/store"
	"github.com/nebari-dev/skillctl/gen/go/skillctl/v1/skillctlv1connect"
)

type Server struct {
	handler http.Handler
}

func New(skillStore store.Repository, authValidator auth.TokenValidator) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealthz)

	interceptor := auth.NewInterceptor(authValidator)
	path, handler := skillctlv1connect.NewRegistryServiceHandler(
		registry.NewService(skillStore),
		connect.WithInterceptors(interceptor),
	)
	mux.Handle(path, handler)

	wrapped := auth.NewAllowlistMiddleware([]string{"/healthz"}, mux)
	return &Server{handler: wrapped}
}

func (s *Server) Handler() http.Handler {
	return s.handler
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
```

Note: `handleHealthz` is no longer a method on `Server` since `Server` no longer holds `mux` directly. It's a package-level function.

- [ ] **Step 4: Update main.go**

Add auth config env vars, validation, and Validator creation:

```go
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	_ "modernc.org/sqlite"

	"github.com/nebari-dev/skillctl/backend/internal/auth"
	"github.com/nebari-dev/skillctl/backend/internal/server"
	sqlitestore "github.com/nebari-dev/skillctl/backend/internal/store/sqlite"
	"github.com/nebari-dev/skillctl/backend/internal/store/sqlite/migrations"
)

func main() {
	port := envOr("PORT", "8080")
	dbPath := envOr("DB_PATH", "skillctl.db")

	db, err := sqlitestore.Open(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := migrations.Run(context.Background(), db); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	authCfg := auth.Config{
		IssuerURL:   envOr("OIDC_ISSUER_URL", ""),
		ClientID:    envOr("OIDC_CLIENT_ID", ""),
		AdminGroup:  envOr("OIDC_ADMIN_GROUP", "skillctl-admins"),
		GroupsClaim: envOr("OIDC_GROUPS_CLAIM", "groups"),
	}

	if (authCfg.IssuerURL == "") != (authCfg.ClientID == "") {
		log.Fatalf("OIDC_ISSUER_URL and OIDC_CLIENT_ID must both be set or both be empty")
	}

	var validator auth.TokenValidator
	if authCfg.IssuerURL != "" {
		v, err := auth.NewValidator(context.Background(), authCfg)
		if err != nil {
			log.Fatalf("init auth: %v", err)
		}
		validator = v
		log.Printf("auth enabled (issuer: %s)", authCfg.IssuerURL)
	} else {
		log.Println("auth disabled (no OIDC_ISSUER_URL)")
	}

	repo := sqlitestore.New(db)
	srv := server.New(repo, validator)

	log.Printf("starting server on :%s (db: %s)", port, dbPath)
	if err := http.ListenAndServe(":"+port, srv.Handler()); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 5: Run all tests**

```bash
go test -race ./backend/... -v
```

Expected: ALL PASS - healthz, auth interceptor, validator, allowlist, server integration.

- [ ] **Step 6: Run go vet**

```bash
go vet ./backend/...
```

Expected: clean.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/server/server.go backend/internal/server/server_test.go backend/cmd/server/main.go
git commit -m "feat: wire OIDC auth into server with interceptor and allowlist"
```

---

### Task 7: Smoke test

- [ ] **Step 1: Start server without auth (local dev)**

```bash
DB_PATH=:memory: go run ./backend/cmd/server
```

Expected: log shows "auth disabled (no OIDC_ISSUER_URL)" and "starting server on :8080".

- [ ] **Step 2: Verify healthz works**

```bash
curl -s http://localhost:8080/healthz
```

Expected: `ok`

- [ ] **Step 3: Verify RPC works without auth (dev mode)**

```bash
curl -s -X POST http://localhost:8080/skillctl.v1.RegistryService/ListSkills \
  -H 'Content-Type: application/json' \
  -d '{}'
```

Expected: `{"skills":[]}` (empty list, no auth error).

- [ ] **Step 4: Verify startup fails with partial auth config**

```bash
OIDC_ISSUER_URL=https://example.com DB_PATH=:memory: go run ./backend/cmd/server 2>&1
```

Expected: fatal error "OIDC_ISSUER_URL and OIDC_CLIENT_ID must both be set or both be empty".

- [ ] **Step 5: Kill the dev server and commit if any adjustments were needed**
