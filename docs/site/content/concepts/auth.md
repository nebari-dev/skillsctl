---
title: "Auth model"
weight: 30
---

# Auth model

SkillsCtl uses OIDC for authentication. The server validates tokens; the CLI gets tokens via the RFC 8628 device flow. Neither component requires manual OIDC configuration from the user.

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
6. The CLI stores the access token and ID token at `~/.config/skillsctl/credentials.json` with `0600` permissions

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

Tokens are stored at `~/.config/skillsctl/credentials.json`. The file is written with `0600` permissions (readable only by the owner). The file contains the access token and ID token as returned by the OIDC provider.

SkillsCtl does not currently handle refresh tokens. When the token expires, re-run `skillsctl auth login` to get a new one. Most OIDC providers issue tokens valid for at least an hour; some issue longer-lived tokens.

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
