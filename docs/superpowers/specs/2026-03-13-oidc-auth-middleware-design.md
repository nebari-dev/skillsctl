# OIDC Auth Middleware Design

**Date:** 2026-03-13
**Status:** Approved
**Scope:** Read-tier authentication for backend ConnectRPC endpoints

## Context

The skillctl backend needs to authenticate incoming requests using OIDC tokens. The system supports three auth tiers (read, write, admin), but only read auth is needed now since the current RPCs (ListSkills, GetSkill) only require a valid identity.

The design is provider-agnostic - it works with any standard OIDC provider (Keycloak, Okta, Dex, etc.). Domain filtering is the IdP's responsibility, not the server's.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| OIDC library | `coreos/go-oidc/v3` | Handles discovery, JWKS caching, and token verification with minimal code. Battle-tested. |
| Auth integration | ConnectRPC unary interceptor | Idiomatic for the framework, composes with future interceptors (logging, rate limiting). Only unary RPCs exist currently; streaming support can be added when streaming RPCs are introduced. |
| Unauthenticated paths | HTTP allowlist middleware wrapping entire mux | Secure-by-default: new endpoints require auth unless explicitly allowlisted |
| Local dev mode | Nil validator disables auth | No OIDC provider needed to run server locally |
| Testability | `TokenValidator` interface | Interceptor accepts interface, enabling stub-based unit tests |
| AllowedDomain | Dropped (YAGNI) | Domain filtering is the OIDC provider's concern |
| Groups claim name | Configurable via `GroupsClaim` field | Different OIDC providers use different claim names (`groups`, `roles`, `cognito:groups`). Defaults to `"groups"`. |

## Architecture

### Components

```
Request
  |
  v
AllowlistMiddleware (/healthz -> skip auth, exact match only)
  |
  v
http.ServeMux
  |
  +-- /healthz (no auth, allowlisted)
  +-- /skillctl.v1.RegistryService/* (ConnectRPC)
  |     |
  |     v
  |   AuthInterceptor (extract Bearer token, validate, inject claims)
  |     |
  |     v
  |   RPC Handler (claims available via context)
  |
  +-- (any other path) -> 404 from mux, no auth check needed
```

**Note on allowlist behavior:** The allowlist uses exact path matching (not prefix). A request to `/healthz-other` would NOT be allowlisted. Paths that are neither allowlisted nor registered in the mux receive a 404 from the mux itself. Auth is enforced by the ConnectRPC interceptor on RPC routes only. This is acceptable because:
- Non-RPC, non-allowlisted paths return 404 (no data leakage)
- All RPC routes go through the interceptor (secure by default)
- New RPC endpoints automatically get auth via the interceptor

### Files

```
backend/internal/auth/
  config.go       - Config struct (IssuerURL, ClientID, AdminGroup, GroupsClaim)
  validator.go    - TokenValidator interface, Validator implementation, Claims struct
  context.go      - WithClaims/ClaimsFromContext context helpers
  interceptor.go  - ConnectRPC unary interceptor
  allowlist.go    - HTTP allowlist middleware
```

### Config

```go
type Config struct {
    IssuerURL   string // OIDC discovery URL
    ClientID    string // expected audience claim
    AdminGroup  string // group name for future admin checks
    GroupsClaim string // JWT claim name for group membership, defaults to "groups"
}
```

Configured via environment variables:
- `OIDC_ISSUER_URL` - required in production, empty disables auth (local dev)
- `OIDC_CLIENT_ID` - required when issuer is set
- `OIDC_ADMIN_GROUP` - defaults to `"skillctl-admins"`
- `OIDC_GROUPS_CLAIM` - defaults to `"groups"`

**Startup validation:** `main.go` must validate that `OIDC_ISSUER_URL` and `OIDC_CLIENT_ID` are either both set or both empty. If only one is set, the server must `log.Fatalf` with a clear error message. This prevents accidentally running in production with auth misconfigured.

### TokenValidator interface

```go
type TokenValidator interface {
    Validate(ctx context.Context, rawToken string) (*Claims, error)
}
```

The interceptor depends on this interface, not the concrete `*Validator`. This follows the project convention (accept interfaces, return concrete types) and enables stub-based testing.

### Validator (production implementation)

```go
type Validator struct {
    provider *oidc.Provider
    verifier *oidc.IDTokenVerifier
    cfg      Config
}

type Claims struct {
    Subject string
    Email   string
    Groups  []string
}
```

`NewValidator(ctx, cfg)` calls `oidc.NewProvider()` which fetches the OIDC discovery doc and sets up automatic JWKS refresh. Fails fast on startup if the OIDC provider is unreachable.

`Validate(ctx, rawToken)`:
1. Verifies JWT signature against cached JWKS (RS256)
2. Checks expiry
3. Checks audience contains ClientID
4. Extracts `sub`, `email`, and the configured groups claim (via `cfg.GroupsClaim`)
5. Returns `*Claims` or error

`IsAdmin(claims)` checks if `cfg.AdminGroup` appears in `claims.Groups`. Implemented and tested, but not wired into any RPC authorization check until admin-tier RPCs are added. Included now because `AdminGroup` is already in Config and the implementation is trivial (a loop over a string slice).

### ConnectRPC interceptor

`NewInterceptor(v TokenValidator) connect.UnaryInterceptorFunc`

1. If validator is nil (local dev): pass through, no claims injected
2. Extract `Authorization: Bearer <token>` from request headers
3. If header is missing, empty, or does not start with `"Bearer "`: return `connect.CodeUnauthenticated`
4. Call `v.Validate(ctx, token)`
5. On success: inject claims into context via `WithClaims()`, call next
6. On failure: return `connect.CodeUnauthenticated`

Edge case: `Authorization: Bearer ` (Bearer prefix with empty token) is treated as malformed and returns `CodeUnauthenticated`.

### Allowlist middleware

`NewAllowlistMiddleware(allowedPaths []string, next http.Handler) http.Handler`

Uses **exact path matching** against the allowlist. Matching requests pass through to `next` directly. Non-matching requests also pass through to `next` (the mux), where ConnectRPC routes hit the interceptor and unknown routes get a 404.

Initial allowlist: `["/healthz"]`

### Server wiring

```go
// server.go
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
```

```go
// main.go
authCfg := auth.Config{
    IssuerURL:   env("OIDC_ISSUER_URL", ""),
    ClientID:    env("OIDC_CLIENT_ID", ""),
    AdminGroup:  env("OIDC_ADMIN_GROUP", "skillctl-admins"),
    GroupsClaim: env("OIDC_GROUPS_CLAIM", "groups"),
}

// Validate: both or neither must be set
if (authCfg.IssuerURL == "") != (authCfg.ClientID == "") {
    log.Fatalf("OIDC_ISSUER_URL and OIDC_CLIENT_ID must both be set or both be empty")
}

var validator auth.TokenValidator
if authCfg.IssuerURL != "" {
    v, err := auth.NewValidator(ctx, authCfg)
    if err != nil {
        log.Fatalf("init auth: %v", err)
    }
    validator = v
}

srv := server.New(repo, validator)
```

## Testing

All tests follow the project convention of table-driven tests.

### Validator tests (fake OIDC server)

An `httptest.Server` serves:
- `/.well-known/openid-configuration` - discovery doc pointing at the test server
- `/jwks` - JWKS endpoint with a test RS256 public key

Test JWTs are minted with the matching RSA private key (2048-bit, RS256 algorithm).

| Test case | Token setup | Expected result |
|-----------|-------------|-----------------|
| valid token with all claims | signed, not expired, correct audience | Claims returned with Subject, Email, Groups |
| expired token | signed, expired 1 hour ago | error |
| wrong audience | signed, audience = "other-client" | error |
| missing email claim | signed, no email in payload | Claims returned, Email is empty string |
| groups from custom claim | signed, groups in "roles" claim | Groups populated when GroupsClaim = "roles" |

### Interceptor tests (stub TokenValidator)

Table-driven with columns: `name`, `validator` (nil or stub), `authHeader`, `wantCode`.

| Test case | Validator | Auth header | Expected |
|-----------|-----------|-------------|----------|
| valid token | stub returning claims | `Bearer valid-token` | RPC succeeds, claims in context |
| missing header | stub | (empty) | `CodeUnauthenticated` |
| malformed prefix | stub | `Basic xyz` | `CodeUnauthenticated` |
| empty bearer token | stub | `Bearer ` | `CodeUnauthenticated` |
| validator error | stub returning error | `Bearer bad-token` | `CodeUnauthenticated` |
| nil validator (dev mode) | nil | (empty) | RPC succeeds, no claims in context |

### Allowlist tests

| Test case | Path | Expected |
|-----------|------|----------|
| allowlisted path | `/healthz` | passes through, handler called |
| non-allowlisted path | `/other` | continues to next handler |
| partial match not allowlisted | `/healthz-other` | continues to next handler (exact match only) |

### Server integration tests

- Existing `TestHealthz` continues to pass (no auth required)
- New test: RPC without token returns `CodeUnauthenticated` when validator is configured
- New test: RPC succeeds with nil validator (local dev mode)

## Future extensions

When write and admin RPCs are added:
- Write tier: a second interceptor (or extended logic) checks `X-Push-Token` header against configured push tokens
- Admin tier: interceptor calls `validator.IsAdmin(claims)` to check group membership
- Claims are already in context, so no plumbing changes needed
- If streaming RPCs are added, a streaming interceptor will be needed alongside the unary one
