# CLI OIDC Auth Design Spec

## Goal

Add OIDC device flow authentication to the CLI so it can authenticate against a production server, with zero client-side auth configuration required.

## Server: Auth Config Endpoint

New unauthenticated HTTP endpoint `GET /auth/config`.

**When OIDC is configured** (OIDC_ISSUER_URL and OIDC_CLIENT_ID are set):
```json
{
  "enabled": true,
  "issuer_url": "https://keycloak.example.com/realms/myrealm",
  "client_id": "skillsctl-cli"
}
```

**When OIDC is not configured:**
```json
{
  "enabled": false
}
```

Behavior:
- Registered on the server mux as a plain HTTP handler (not a ConnectRPC method) since it's a simple JSON GET, not an RPC.
- Since `/auth/config` is a plain HTTP handler registered on the mux, the ConnectRPC interceptor never touches it (the interceptor only runs for ConnectRPC handlers). No allowlist change needed.
- Uses the existing `OIDC_ISSUER_URL` and `OIDC_CLIENT_ID` env vars. No new server configuration.
- Returns `Content-Type: application/json`.

## CLI: auth login

```
$ skillsctl auth login
Fetching auth config from http://localhost:8080...

Go to: https://keycloak.example.com/realms/myrealm/device
Enter code: ABCD-EFGH

Waiting for authentication...
Logged in as user@example.com
```

Flow:
1. Fetch `GET <api_url>/auth/config`. The `api_url` comes from the normal precedence chain (--api-url flag > env > config file > default). If the user hasn't configured anything, it defaults to `http://localhost:8080`. If `enabled` is false, print "Server does not require authentication." and exit 0.
2. If `issuer_url` is not HTTPS (and not localhost), print a warning: "Warning: OIDC issuer is not using HTTPS." Continue anyway - the user may have a valid reason.
3. Fetch OIDC discovery document from `<issuer_url>/.well-known/openid-configuration` to get `device_authorization_endpoint` and `token_endpoint`. If the discovery doc lacks `device_authorization_endpoint`, print "OIDC provider does not support device flow" and exit 1.
4. POST to `device_authorization_endpoint` with `client_id=<client_id>&scope=openid email profile`. No `client_secret` is sent - this is a public client (RFC 8628 Section 3.1). Scope intentionally omits `offline_access` since we don't store refresh tokens.
5. Print `verification_uri_complete` if available (includes code in URL), otherwise print `verification_uri` and `user_code` separately. Output to stderr (interactive prompt).
6. Poll `token_endpoint` every `interval` seconds (from device auth response, default 5) until:
   - Success: token response received
   - `authorization_pending`: continue polling
   - `slow_down`: increase interval by 5 seconds
   - `expired_token`: print "Authentication timed out. Try again." and exit 1
   - `access_denied`: print "Authentication denied." and exit 1
7. Extract `id_token` from token response. Decode JWT payload (base64, no verification - the server will verify) to extract `email` claim and `exp` claim. The email is for display only, not security-critical - an attacker who can MITM the token endpoint could forge the displayed email, but the server would reject the forged token.
8. Save token and expiry to `~/.config/skillsctl/credentials.json` with 0600 permissions. Create parent directory with 0700 permissions via `os.MkdirAll` if needed.
9. Print "Logged in as <email>" to stdout (primary command output).

## CLI: auth status

```
$ skillsctl auth status
Logged in as user@example.com
Token expires at 2026-03-17 12:00:00 UTC

$ skillsctl auth status
Not logged in
```

Behavior:
- Read `credentials.json`. If missing or malformed, print "Not logged in" and exit 1.
- If token exists, read the stored `expiry` field (authoritative source for expiry checking - no need to re-decode JWT).
- If expired, print "Session expired. Run 'skillsctl auth login' to re-authenticate." and exit 1.
- If valid, decode JWT to get email for display, print status, exit 0.

Exit codes: 0 = valid token, 1 = not logged in or expired. This supports scripting like `skillsctl auth status || skillsctl auth login`.

## CLI: auth logout

```
$ skillsctl auth logout
Logged out
```

Behavior:
- Delete `credentials.json`.
- Print "Logged out" regardless of whether the file existed.

## Token Storage

File: `~/.config/skillsctl/credentials.json`

```json
{
  "id_token": "eyJhbGciOiJSUzI1NiIs...",
  "expiry": "2026-03-17T12:00:00Z"
}
```

- Only the ID token and its expiry are stored. The `expiry` field is extracted from the JWT `exp` claim during login and stored as the authoritative source for expiry checks.
- No refresh token logic. When the token expires, the user runs `auth login` again.
- File permissions: 0600 (owner read/write only). Directory permissions: 0700.
- The credentials path follows the config directory: `~/.config/skillsctl/`.
- If the file is malformed or unreadable, treat as "no token" (return empty string from `loadCachedToken`). Do not error.

## Token Attachment

Rather than duplicating token loading in every command, add a shared `getClient()` helper in `root.go` (matching the existing `getAPIURL()` pattern):

```go
func getClient() *api.Client {
    return api.NewClient(getAPIURL(), api.WithToken(loadCachedToken()))
}
```

All commands use `getClient()` instead of `api.NewClient(getAPIURL())`. The `loadCachedToken()` function reads `credentials.json`, checks the stored `expiry` field, returns the ID token string if valid or empty string otherwise.

The API client constructor changes from `NewClient(baseURL string)` to accept options:

```go
func NewClient(baseURL string, opts ...ClientOption) *Client
type ClientOption func(*Client)
func WithToken(token string) ClientOption
```

When a token is set, the client creates an `http.Client` with a custom `RoundTripper` that injects the `Authorization: Bearer <token>` header on every request.

When the server returns `CodeUnauthenticated`, the existing error mapping in `publish.go` already prints the right message. The same mapping should be applied in `install.go` and `explore.go` for consistency.

## OIDC Discovery

The CLI fetches two documents:
1. `/auth/config` from the skillsctl server (issuer URL + client ID)
2. `<issuer_url>/.well-known/openid-configuration` from the OIDC provider (device auth endpoint + token endpoint)

Both are simple HTTP GETs returning JSON. No external OIDC library needed for the CLI - just `net/http` and `encoding/json`. The JWT is decoded (not verified) using base64 to extract claims for display only.

## Files to Create/Modify

### New files:
- `cli/internal/auth/device_flow.go` - OIDC device flow implementation (discovery, device auth, token polling)
- `cli/internal/auth/device_flow_test.go` - tests with mock OIDC + mock skillsctl server
- `cli/internal/auth/credentials.go` - token cache read/write/delete, expiry checking
- `cli/internal/auth/credentials_test.go` - tests including malformed file, expired token
- `cli/cmd/auth.go` - auth login/status/logout commands, addAuthCmd registration
- `cli/cmd/auth_test.go` - tests

### Modified files:
- `backend/internal/server/server.go` - add /auth/config handler
- `backend/internal/server/server_test.go` - test /auth/config with and without OIDC
- `cli/internal/api/client.go` - add WithToken option, token RoundTripper
- `cli/internal/api/client_test.go` - test token attachment
- `cli/cmd/root.go` - register auth commands via addAuthCmd(rootCmd), add getClient() helper
- `cli/cmd/explore.go` - use getClient() instead of api.NewClient(getAPIURL())
- `cli/cmd/publish.go` - use getClient()
- `cli/cmd/install.go` - use getClient()

## Error Handling

| Scenario | Message | Exit |
|----------|---------|------|
| Server unreachable during `auth login` | "Cannot reach server at <url>: <error>" | 1 |
| Auth disabled on server | "Server does not require authentication." | 0 |
| OIDC discovery fails | "Cannot reach OIDC provider at <issuer>: <error>" | 1 |
| Device flow not supported by provider | "OIDC provider does not support device flow" | 1 |
| Non-HTTPS issuer URL (not localhost) | "Warning: OIDC issuer is not using HTTPS." (continues) | - |
| User denies auth | "Authentication denied." | 1 |
| Device code expires | "Authentication timed out. Try again." | 1 |
| `auth status` - not logged in | "Not logged in" | 1 |
| `auth status` - expired | "Session expired. Run 'skillsctl auth login' to re-authenticate." | 1 |
| Expired token on any command | "Not authenticated. Run 'skillsctl auth login' first." | 1 |

## Testing Strategy

- **Device flow**: Two mock HTTP servers - one serving `/auth/config` (mock skillsctl server) and one serving `/.well-known/openid-configuration` + device/token endpoints (mock OIDC provider). Test the polling loop with different responses (pending, success, denied, expired, slow_down). Use a configurable poll function or minimal interval to avoid `time.Sleep` in tests.
- **Credentials**: Test read/write/delete with temp directory. Test expiry checking (valid, expired, missing file, malformed JSON all return correct results).
- **Auth commands**: Test login against mock servers, test status output and exit codes, test logout deletes file.
- **Token attachment**: Test that API client sends Authorization header when token is set, and omits it when not set.
- **Server /auth/config**: Test response with OIDC configured and without.
- **getClient() wiring**: Test that existing commands (explore, publish, install) continue to work with the refactored client creation.

## Non-goals

- Token refresh (user re-runs `auth login` when expired)
- Browser auto-open (print URL, user opens manually)
- Multiple OIDC provider support (one provider per server)
- Client-side token verification (server verifies)
- `offline_access` scope / refresh tokens
