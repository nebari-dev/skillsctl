---
title: "Configuration reference"
weight: 40
---

# Configuration reference

The SkillsCtl server is configured entirely through environment variables. There is no configuration file.

## Environment variables

### Server

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | TCP port the server listens on |
| `DB_PATH` | `./skillsctl.db` | Path to the SQLite database file |
| `DEV_MODE` | `false` | Set to "true" to run without authentication. Required when OIDC is not configured. |
| `APP_VERSION` | `0.0.0` | Application version, used for seeding default skills. |

The database is created automatically if it does not exist. Migrations run on startup.

### Upload limits

| Variable | Default | Description |
|----------|---------|-------------|
| `LIMITS_MAX_PACKED_BYTES` | `5242880` (5 MB) | Maximum size of the tar.gz uploaded by `skillsctl publish`. |
| `LIMITS_MAX_TOTAL_BYTES` | `20971520` (20 MB) | Maximum total uncompressed size of all files in a skill package. |
| `LIMITS_MAX_FILES` | `100` | Maximum number of files in a skill package. |
| `LIMITS_MAX_FILE_BYTES` | `1048576` (1 MB) | Maximum size of any single file in a skill package. |

The server re-validates every uploaded tarball against these limits and rejects packages that exceed them. It also rejects tarballs that are missing `SKILL.md` at the root, contain symlinks or hardlinks, use absolute paths, or include path components that traverse parent directories.

The current limits are published at `GET /limits` as JSON:

```json
{
  "MaxPackedBytes": 5242880,
  "MaxTotalBytes": 20971520,
  "MaxFiles": 100,
  "MaxFileBytes": 1048576
}
```

This endpoint is unauthenticated and is used by the CLI to surface human-readable errors before uploading.

## GET /status

Reports the release this binary was built from and what the startup seed did:

```json
{
  "buildVersion": "0.2.3",
  "seed": {
    "version": "0.2.3",
    "skills": [
      {"name": "skillsctl-usage", "version": "0.2.3", "outcome": "seeded"}
    ]
  }
}
```

`outcome` is one of:

| Outcome | Meaning |
|---------|---------|
| `seeded` | This run published the version. |
| `already-present` | The version existed, so nothing was written. Whatever content is in the registry was published by whichever binary got there first, not necessarily this one. |
| `not-owner` | Another publisher owns the skill, so the embedded copy can no longer update it. |

When the binary's stamped version disagrees with `APP_VERSION`, seeding is declined and `seed.declined` carries the reason instead of a version. That happens when a pod from an older image reads a configmap a newer release already updated; publishing under that version would make it permanently wrong, because versions are immutable.

Use this endpoint after a deploy to confirm the release actually republished its embedded skills. This endpoint is unauthenticated.

### OIDC

| Variable | Required | Description |
|----------|----------|-------------|
| `OIDC_ISSUER_URL` | No | OIDC issuer URL. If unset, the server runs in dev mode with auth disabled. |
| `OIDC_CLIENT_ID` | No | OIDC client ID. Returned to the CLI via `/auth/config` so the CLI can self-configure. |
| `OIDC_ADMIN_GROUP` | No | Group name in the JWT `groups` claim (or `OIDC_GROUPS_CLAIM`) that grants admin access. |
| `OIDC_GROUPS_CLAIM` | No | JWT claim name containing group membership. Default: `groups`. |
| `OIDC_DEVICE_CLIENT_ID` | No | Public client ID for CLI device flow. Returned to CLI via /auth/config. |

## Dev mode

When `DEV_MODE` is set to `true`, the server starts without authentication. If `OIDC_ISSUER_URL` is not set and `DEV_MODE` is not `true`, the server exits with an error.

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
