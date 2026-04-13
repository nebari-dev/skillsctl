---
title: "Auth model"
weight: 30
---

# Auth model

SkillsCtl uses OIDC for authentication. The server validates tokens; the CLI gets tokens via the RFC 8628 device flow. The CLI discovers OIDC settings from the server automatically - users never configure OIDC on the client side. (The server itself still needs OIDC environment variables; see [Server configuration]({{< relref "/server/configuration" >}}).)

## Server-side validation

The server validates every authenticated request by verifying the JWT signature against the issuer's JWKS. It fetches the JWKS from `{OIDC_ISSUER_URL}/.well-known/openid-configuration` at startup.

Token validation checks:
- Signature (using the issuer's public keys)
- Expiry
- Issuer and audience claims

The server does not maintain sessions. Each request carries the token directly.

## Zero-config CLI discovery

When a user runs `skillsctl auth login`, the CLI first calls `GET /auth/config` on the configured server. This endpoint returns the OIDC issuer URL and client ID. The CLI uses those values to start the device flow - the user does not need to configure OIDC settings manually.

This means you can point the CLI at any SkillsCtl server and authentication just works. The server is the single source of truth for OIDC configuration.

## Device flow (RFC 8628)

The CLI uses the RFC 8628 device authorization flow:

1. The CLI calls the OIDC device authorization endpoint with the client ID
2. The OIDC provider returns a user code and verification URL
3. The CLI prints the URL and code to the terminal
4. The user opens the URL in a browser and enters the code
5. The CLI polls the token endpoint until the user completes authorization
6. The CLI stores the ID token (plus refresh token and refresh metadata, when returned) at `~/.config/skillsctl/credentials.json` with `0600` permissions

Example flow:

```
$ skillsctl auth login

To authorize, open: https://keycloak.example.com/device
Enter code: ABCD-EFGH

Waiting for authorization...
Logged in as alice@example.com
```

The device flow works without a redirect URI, making it suitable for CLI tools where a browser redirect back to `localhost` is not reliable.

## Token storage

Tokens are stored at `~/.config/skillsctl/credentials.json`. The file is written with `0600` permissions (readable only by the owner) inside a `0700` parent directory. The file contains the ID token, its expiry, and - when the OIDC provider returns one - the refresh token plus the token endpoint and client ID needed to use it.

## Silent token refresh

When an authenticated command runs and the cached ID token is expired (or within 60 seconds of expiring), the CLI silently exchanges the refresh token for a fresh ID token using the OAuth 2.0 `refresh_token` grant (RFC 6749 §6). The renewed credentials are written back to `credentials.json` before the command proceeds. If the provider rotates the refresh token (Keycloak does by default), the new one is stored; otherwise the existing refresh token is kept.

Only when the refresh itself fails - for example, because the OIDC session timed out or an admin revoked the token - does the CLI fall back to reporting "not authenticated" and asking for a new `skillsctl auth login`. In typical use you should log in once per session lifespan (hours, not minutes), not once per command.

Check token status:

```bash
skillsctl auth status
```

Log out (deletes the credentials file):

```bash
skillsctl auth logout
```

## Ownership and identity

Ownership of skills is keyed on the OIDC subject (`sub` claim), not on email. Subjects are stable across email changes - if a user changes their email in the OIDC provider, they retain ownership of skills they published. Email addresses are displayed for readability but are not used for access control decisions.

## Admin access

Admin operations require membership in the group specified by `OIDC_ADMIN_GROUP` on the server. Group membership is read from the JWT claim named by `OIDC_GROUPS_CLAIM` (default: `groups`). If the claim is present and contains the admin group name, the request is treated as an admin request.

No separate admin token or API key is needed - standard OIDC tokens carry group membership.

## Dev mode

When `OIDC_ISSUER_URL` is not set on the server, auth is disabled. All requests succeed and a default identity is injected. The `/auth/config` endpoint returns empty values. The CLI does not prompt for login when talking to a dev-mode server.

## Next steps

- [Versioning and ownership]({{< relref "/concepts/versioning" >}}) - how OIDC subjects relate to skill ownership
- [Configuration reference]({{< relref "/server/configuration" >}}) - OIDC environment variables
