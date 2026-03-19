---
title: "Configuration reference"
weight: 40
---

# Configuration reference

The skillsctl server is configured entirely through environment variables. There is no configuration file.

## Environment variables

### Server

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | TCP port the server listens on |
| `DB_PATH` | `./skillsctl.db` | Path to the SQLite database file |

The database is created automatically if it does not exist. Migrations run on startup.

### OIDC

| Variable | Required | Description |
|----------|----------|-------------|
| `OIDC_ISSUER_URL` | No | OIDC issuer URL. If unset, the server runs in dev mode with auth disabled. |
| `OIDC_CLIENT_ID` | No | OIDC client ID. Returned to the CLI via `/auth/config` so the CLI can self-configure. |
| `OIDC_ADMIN_GROUP` | No | Group name in the JWT `groups` claim (or `OIDC_GROUPS_CLAIM`) that grants admin access. |
| `OIDC_GROUPS_CLAIM` | No | JWT claim name containing group membership. Default: `groups`. |

## Dev mode

When `OIDC_ISSUER_URL` is not set, the server starts in dev mode:

- All requests are accepted without credentials
- A default identity is injected for ownership tracking
- The `/auth/config` endpoint returns empty OIDC settings

Dev mode is intended for local development only. Do not run it in production.

## OIDC setup

To enable authentication, set at minimum `OIDC_ISSUER_URL` and `OIDC_CLIENT_ID`. The server fetches the JWKS from the issuer's discovery endpoint (`{OIDC_ISSUER_URL}/.well-known/openid-configuration`) and validates token signatures on each request.

Example with Keycloak:

```bash
export OIDC_ISSUER_URL=https://keycloak.example.com/realms/myrealm
export OIDC_CLIENT_ID=skillsctl
export OIDC_ADMIN_GROUP=platform-admins
```

Example with Auth0:

```bash
export OIDC_ISSUER_URL=https://your-tenant.auth0.com/
export OIDC_CLIENT_ID=your-client-id
export OIDC_ADMIN_GROUP=skillsctl-admins
export OIDC_GROUPS_CLAIM=https://your-tenant.auth0.com/groups
```

Auth0 uses namespaced custom claims. Set `OIDC_GROUPS_CLAIM` to the full claim name you configured.

### Admin access

The admin group controls who can perform admin operations (currently: approving externally-sourced skills). Set `OIDC_ADMIN_GROUP` to the name of a group in your OIDC provider. Users whose JWT contains that group name in the groups claim are treated as admins.

If `OIDC_ADMIN_GROUP` is not set, admin endpoints are inaccessible to all users.

### What the CLI needs

The CLI discovers OIDC settings from the server automatically by calling `GET /auth/config`. The response includes `OIDC_ISSUER_URL` and `OIDC_CLIENT_ID`. The CLI uses these to initiate the device flow without any manual configuration. Users just run `skillsctl auth login`.

## SQLite settings

The server configures SQLite with these pragmas on every connection:

- `WAL` journal mode - allows concurrent reads during writes
- `busy_timeout=5000` - retries for up to 5 seconds before returning a busy error
- `foreign_keys=ON` - enforces referential integrity

These settings are not configurable. The busy timeout of 5 seconds is appropriate for the expected write rate of a skill registry (publishing is infrequent).

## Database size

Skills content is stored as a BLOB in the `skill_versions` table. At 10KB average content size, 1Gi of disk holds approximately 50,000 skill versions. The PVC default of 1Gi is sufficient for most deployments; increase `persistence.size` in the Helm values if needed.

## Kubernetes (Helm)

When deploying with Helm, set OIDC values through the chart's `oidc` values block. The chart maps these to the corresponding environment variables in the pod spec. See [Kubernetes deployment]({{< relref "/server/kubernetes" >}}) for the full values reference.
