# skillctl — Complete Work Plan v2

**For:** Claude Code  
**Project:** A cloud-native CLI tool + backend registry for discovering, installing, and publishing Claude Code skills, with federated marketplace support and admin-controlled external skill whitelisting  
**Last updated:** 2026-03-11  
**Changes from v1:** Two-repo split, K8s-native storage (PostgreSQL + Valkey), OCI artifact skill storage, generic OIDC device flow, Helm chart on ghcr.io, Nebari NebariApp CRD opt-in

---

## Table of Contents

1. [Two-Repo Split](#1-two-repo-split)
2. [Architecture Overview](#2-architecture-overview)
3. [Repo 1: `skillctl` (OSS tool)](#3-repo-1-skillctl-oss-tool)
   - 3.1 Repository Structure
   - 3.2 Technology Decisions
   - 3.3 Phase 1 — Protobuf & ConnectRPC
   - 3.4 Phase 2 — Backend Service
   - 3.5 Phase 3 — Helm Chart
   - 3.6 Phase 4 — CLI
   - 3.7 Phase 5 — CI/CD Pipelines
4. [Repo 2: `skillctl-deploy` (org GitOps)](#4-repo-2-skillctl-deploy-org-gitops)
   - 4.1 Repository Structure
   - 4.2 ArgoCD Application Manifests
   - 4.3 Org Values Overlay
   - 4.4 NebariApp CRD
5. [Federation & Marketplace Management](#5-federation--marketplace-management)
6. [Auth Design](#6-auth-design)
7. [Dogfood Skill](#7-dogfood-skill)
8. [Outstanding Decisions & Constraints](#8-outstanding-decisions--constraints)

---

## 1. Two-Repo Split

| Repo | Audience | What it contains |
|---|---|---|
| `yourorg/skillctl` | Public / OSS | CLI binary, backend server, Helm chart, proto definitions, CI/CD, docs |
| `yourorg/skillctl-deploy` | Internal / private | ArgoCD Application manifests, org-specific Helm values, Keycloak client config, NebariApp CRD overlay |

### Why this split

`skillctl` is a generic tool — any team on any K8s cluster with any OIDC provider can deploy it. It ships with no org-specific configuration. The `skillctl-deploy` repo is where OpenTeams-specific configuration lives: Keycloak issuer URL, realm, admin group name, ingress hostname, storage class names, and the NebariApp CRD that wires it into the Nebari platform.

This means `skillctl` can be open-sourced without leaking internal infrastructure details, and the org repo can reference pinned chart versions from ghcr.io rather than building anything itself.

### Deployment flow

```
skillctl repo (CI)
  → builds + pushes ghcr.io/yourorg/skillctl-backend:vX.Y.Z
  → pushes oci://ghcr.io/yourorg/charts/skillctl:vX.Y.Z

skillctl-deploy repo (ArgoCD)
  → Application manifest references chart vX.Y.Z
  → values.yaml points at Keycloak, sets storage class, enables NebariApp
  → ArgoCD syncs to cluster
```

---

## 2. Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                      Kubernetes Cluster                             │
│  (Nebari or plain K8s)                                              │
│                                                                     │
│  skillctl namespace                                                 │
│  ├── Deployment: skillctl-server                                    │
│  │     ├── ConnectRPC API                                           │
│  │     ├── In-memory skill cache (warm from PostgreSQL)             │
│  │     ├── Valkey subscriber (cache invalidation pub/sub)           │
│  │     └── Federation poller goroutine                              │
│  │                                                                  │
│  ├── StatefulSet: PostgreSQL (via CloudNativePG operator)           │
│  │     ├── skills, skill_versions tables                            │
│  │     ├── marketplaces, federated_skills tables                    │
│  │     └── audit_log table                                          │
│  │                                                                  │
│  ├── Deployment: Valkey                                             │
│  │     └── Cache invalidation pub/sub channel                       │
│  │         (server publishes on write, all instances invalidate)    │
│  │                                                                  │
│  └── OCI Registry (external or in-cluster)                         │
│        └── ghcr.io/yourorg/skills/{name}:{version}                 │
│            (skill archives stored as OCI artifacts via oras)        │
│                                                                     │
│  [If Nebari] nebari-operator watches NebariApp CRD:                 │
│  ├── Envoy Gateway HTTPRoute → skillctl-server Service              │
│  ├── cert-manager TLS certificate                                   │
│  └── Keycloak OIDC SecurityPolicy on the route                     │
└─────────────────────────────────────────────────────────────────────┘
          │  HTTPS / ConnectRPC
┌─────────┴──────────────────────────────────────────────────────────┐
│  skillctl CLI (Go binary)                                           │
│    linux/amd64, linux/arm64                                         │
│    darwin/amd64, darwin/arm64                                       │
│    windows/amd64                                                    │
│                                                                     │
│  Auth: Generic OIDC device flow                                     │
│    Issuer URL, client ID: read from ~/.config/skillctl/config.yaml  │
│    Configured by org during onboarding (or pre-set in binary        │
│    build via ldflags for org-specific distribution)                 │
└────────────────────────────────────────────────────────────────────┘
          │  Federated fetch (server-side, scheduled)
┌─────────┴──────────────────────────────────────────────────────────┐
│  Whitelisted External Marketplaces                                  │
│    github.com/anthropics/skills                                     │
│    github.com/obra/superpowers                                      │
│    ... (admin-controlled)                                           │
└────────────────────────────────────────────────────────────────────┘
```

### K8s-native storage design

**PostgreSQL (CloudNativePG)** is the durable source of truth. All skill metadata, versions, marketplace configs, and audit logs live here.

**Valkey** handles cache invalidation only — not primary storage. When a server instance writes to PostgreSQL (skill publish, marketplace update), it publishes an invalidation message to a Valkey channel. All server instances (there may be multiple replicas) subscribe and drop their in-memory cache entry, triggering a reload from PostgreSQL. This replaces the Firestore real-time listener from v1 with a standard K8s-native pub/sub pattern.

**OCI registry** stores skill archives. When a developer runs `skillctl push`, the server packages the skill as an OCI artifact and pushes it to the configured registry using the `oras` library. On `skillctl install`, the server generates a short-lived pull token and returns the OCI reference + token to the CLI. The CLI pulls directly using the oras client library. This reuses your existing container registry infrastructure — no separate object store needed.

---

## 3. Repo 1: `skillctl` (OSS Tool)

### 3.1 Repository Structure

```
skillctl/
├── .github/
│   ├── workflows/
│   │   ├── ci-cli.yml           # lint → test → build → e2e → release
│   │   ├── ci-backend.yml       # lint → test → build → e2e → push image
│   │   ├── ci-chart.yml         # helm lint → ct test → push to ghcr.io OCI
│   │   ├── ci-proto.yml         # buf lint + breaking check
│   │   └── ci-docs.yml          # generate + publish to GitHub Pages
│   └── PULL_REQUEST_TEMPLATE.md
│
├── proto/
│   ├── buf.yaml
│   ├── buf.gen.yaml
│   └── skillctl/v1/
│       ├── skill.proto
│       ├── registry.proto
│       ├── federation.proto
│       └── auth.proto
│
├── gen/
│   ├── go/skillctl/v1/
│   └── python/skillctl/v1/
│
├── backend/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── auth/
│   │   │   ├── middleware.go    # OIDC token validation + claims extraction
│   │   │   ├── oidc.go          # Generic OIDC provider: JWKS fetch, token verify
│   │   │   └── groups.go        # Admin group check via OIDC groups claim
│   │   ├── registry/
│   │   │   ├── service.go
│   │   │   ├── cache.go         # In-memory cache + Valkey invalidation subscriber
│   │   │   └── search.go
│   │   ├── federation/
│   │   │   ├── service.go
│   │   │   ├── poller.go
│   │   │   ├── github.go
│   │   │   └── agentskills.go
│   │   ├── store/
│   │   │   ├── postgres.go      # PostgreSQL reads/writes via pgx
│   │   │   ├── migrations/      # SQL migration files (goose)
│   │   │   │   ├── 001_initial.sql
│   │   │   │   ├── 002_federation.sql
│   │   │   │   └── 003_audit_log.sql
│   │   │   └── oci.go           # OCI artifact push/pull via oras-go
│   │   ├── cache/
│   │   │   └── valkey.go        # Valkey pub/sub for cache invalidation
│   │   └── ratelimit/
│   │       └── middleware.go
│   ├── Dockerfile
│   └── .goreleaser-backend.yml
│
├── chart/
│   ├── Chart.yaml
│   ├── values.yaml              # All defaults, no org-specific values
│   ├── values.schema.json       # JSON Schema validation for values
│   ├── templates/
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   ├── serviceaccount.yaml
│   │   ├── configmap.yaml
│   │   ├── secret.yaml          # DB credentials (or external-secrets ref)
│   │   ├── hpa.yaml
│   │   ├── pdb.yaml
│   │   ├── _helpers.tpl
│   │   ├── nebariapp.yaml       # Conditionally rendered: nebari.enabled=true
│   │   └── NOTES.txt
│   ├── crds/                    # NebariApp CRD (only installed if nebari.enabled)
│   │   └── nebariapp-crd.yaml   # Copied from nebari-operator release, versioned
│   └── ci/
│       └── test-values.yaml     # Values used by chart-testing in CI
│
├── cli/
│   ├── main.go
│   ├── cmd/
│   │   ├── root.go
│   │   ├── explore.go
│   │   ├── install.go
│   │   ├── push.go
│   │   ├── auth.go
│   │   ├── update.go
│   │   ├── marketplace.go
│   │   └── docs.go
│   ├── internal/
│   │   ├── api/client.go
│   │   ├── auth/
│   │   │   ├── device_flow.go   # Generic OIDC device flow
│   │   │   └── token_store.go
│   │   ├── oci/
│   │   │   └── pull.go          # oras-go client for pulling skill archives
│   │   ├── skill/
│   │   │   ├── package.go
│   │   │   └── validate.go
│   │   ├── selfupdate/update.go
│   │   └── config/config.go
│   └── .goreleaser.yml
│
├── e2e/
│   ├── cli/
│   │   ├── explore_test.go
│   │   ├── install_test.go
│   │   ├── push_test.go
│   │   └── marketplace_test.go
│   └── backend/
│       └── zap-baseline.yaml
│
├── docs/
│   └── .gitkeep
│
├── skills/
│   └── goreleaser/SKILL.md
│
├── Makefile
├── .golangci.yml
├── .gitleaks.toml
├── VERSION
└── README.md
```

### 3.2 Technology Decisions

| Concern | Choice | Rationale |
|---|---|---|
| RPC | ConnectRPC | Same as v1 — JSON/HTTP compatible, ZAP-friendly |
| Primary store | PostgreSQL via CloudNativePG operator | Durable, queryable, standard K8s pattern. pgx driver in Go. Goose for migrations. |
| Cache invalidation | Valkey pub/sub | Lightweight, K8s-native, handles multi-replica cache coherence. Not primary storage. |
| Skill archive storage | OCI registry via oras-go | Reuses existing container registry. Content-addressed. No separate object store. |
| CLI auth | Generic OIDC device flow | Configurable issuer URL + client ID. Works with Keycloak, Okta, Dex, etc. |
| Backend auth | OIDC token validation | Validates JWT signature against issuer's JWKS. Checks `groups` claim for admin role. No provider-specific SDK. |
| Admin role | OIDC `groups` claim | Admins are members of a group (e.g. `skillctl-admins`) that Keycloak includes in the token's `groups` claim. No separate API call needed at runtime. |
| Helm chart distribution | OCI registry (ghcr.io) | `helm install oci://ghcr.io/yourorg/charts/skillctl --version X.Y.Z` |
| Nebari integration | NebariApp CRD, opt-in via `nebari.enabled=true` | When enabled, renders a NebariApp resource that the nebari-operator picks up for routing + TLS + Keycloak OIDC |
| DB migrations | goose (embedded) | Server runs migrations on startup via goose's embed FS support. No separate init container needed. |
| Container | Scratch final image | Same as v1 — CGO_ENABLED=0, static binary |
| Secrets scanning | Gitleaks | Pre-commit + CI |
| DAST | OWASP ZAP | Post-deploy against running server |
| Release | GoReleaser | CLI binaries + Scoop manifest. Backend container via goreleaser-backend.yml. Helm chart via ci-chart.yml. |

### 3.3 Phase 1 — Protobuf & ConnectRPC

Same buf.yaml and buf.gen.yaml as v1 (Go + Python generation).

The proto service definitions are identical to v1 with one addition: remove any GCP-specific fields (no GCS paths, no Firestore references). The `GetDownloadURL` RPC now returns an OCI reference + short-lived pull token instead of a GCS signed URL:

```protobuf
message GetDownloadURLResponse {
  string oci_ref   = 1;  // e.g. ghcr.io/yourorg/skills/pdf:0.9.1
  string pull_token = 2; // short-lived registry token (TTL: 15 min)
  string digest    = 3;  // sha256 digest for verification
}
```

Breaking change policy: same as v1 (warn pre-1.0, block post-1.0).

### 3.4 Phase 2 — Backend Service

#### Auth middleware (generic OIDC)

```go
// internal/auth/oidc.go

type Config struct {
    IssuerURL   string // e.g. https://keycloak.example.com/realms/myrealm
    ClientID    string
    AdminGroup  string // e.g. "skillctl-admins"
    // AllowedDomain is optional — if set, checks the "email" claim's domain.
    // Leave empty to allow any authenticated user from the OIDC provider.
    AllowedDomain string
}

type Validator struct {
    cfg      Config
    keySet   *jwk.AutoRefresh // auto-refreshes JWKS from issuer
}

func (v *Validator) Validate(ctx context.Context, rawToken string) (*Claims, error) {
    // 1. Fetch + cache JWKS from {IssuerURL}/.well-known/openid-configuration
    // 2. Verify signature, expiry, audience (must include ClientID)
    // 3. If AllowedDomain set: check email domain
    // 4. Extract groups claim → []string
    // 5. Return Claims{Email, Groups, Subject}
}

func (v *Validator) IsAdmin(claims *Claims) bool {
    for _, g := range claims.Groups {
        if g == v.cfg.AdminGroup {
            return true
        }
    }
    return false
}
```

No external API calls for group membership — it comes from the token itself. Keycloak is configured (in the org repo) to include group membership in the `groups` claim. This means admin checks add zero latency.

#### PostgreSQL schema (goose migrations)

```sql
-- 001_initial.sql

CREATE TABLE skills (
    name            TEXT PRIMARY KEY,
    description     TEXT NOT NULL,
    owner           TEXT NOT NULL,
    tags            TEXT[] NOT NULL DEFAULT '{}',
    latest_version  TEXT NOT NULL,
    install_count   BIGINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE skill_versions (
    skill_name    TEXT NOT NULL REFERENCES skills(name),
    version       TEXT NOT NULL,
    oci_ref       TEXT NOT NULL,  -- ghcr.io/yourorg/skills/pdf:0.9.1
    digest        TEXT NOT NULL,  -- sha256 content digest
    published_by  TEXT NOT NULL,
    changelog     TEXT,
    size_bytes    BIGINT,
    published_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (skill_name, version)
);

CREATE INDEX idx_skills_tags ON skills USING GIN(tags);
CREATE INDEX idx_skills_updated ON skills(updated_at DESC);

-- 002_federation.sql

CREATE TABLE marketplaces (
    id                      TEXT PRIMARY KEY,
    display_name            TEXT NOT NULL,
    source_url              TEXT NOT NULL,
    type                    TEXT NOT NULL,  -- GITHUB_REPO, AGENTSKILLS_IO
    mode                    TEXT NOT NULL,  -- ALL, SPECIFIC
    allowed_skills          TEXT[] NOT NULL DEFAULT '{}',
    sync_interval_minutes   INT NOT NULL DEFAULT 60,
    last_synced_at          TIMESTAMPTZ,
    skill_count             INT NOT NULL DEFAULT 0,
    enabled                 BOOLEAN NOT NULL DEFAULT TRUE,
    added_by                TEXT NOT NULL,
    added_at                TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE federated_skills (
    marketplace_id  TEXT NOT NULL REFERENCES marketplaces(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    description     TEXT,
    tags            TEXT[] NOT NULL DEFAULT '{}',
    latest_version  TEXT,
    upstream_url    TEXT NOT NULL,
    allowed         BOOLEAN NOT NULL DEFAULT TRUE,
    synced_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (marketplace_id, name)
);

-- 003_audit_log.sql

CREATE TABLE audit_log (
    id          BIGSERIAL PRIMARY KEY,
    action      TEXT NOT NULL,
    actor       TEXT NOT NULL,
    target_id   TEXT NOT NULL,
    details     JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_log_actor  ON audit_log(actor);
CREATE INDEX idx_audit_log_target ON audit_log(target_id);
CREATE INDEX idx_audit_log_time   ON audit_log(created_at DESC);
```

Goose is embedded in the binary and runs migrations at startup:

```go
// cmd/server/main.go
//go:embed internal/store/migrations/*.sql
var migrations embed.FS

func main() {
    db := mustOpenDB(cfg.DatabaseURL)
    goose.SetBaseFS(migrations)
    goose.Up(db, "internal/store/migrations")
    // ... start server
}
```

#### Valkey cache invalidation

Each server instance maintains an in-memory skill cache loaded from PostgreSQL on startup. When any instance writes (publish, update, federation sync), it publishes to a Valkey channel. All instances subscribe and drop the affected cache entry.

```go
// internal/cache/valkey.go

const invalidationChannel = "skillctl:cache:invalidate"

type Invalidator struct {
    client *valkey.Client
    cache  *registry.Cache
}

// Called after any write to PostgreSQL
func (inv *Invalidator) Publish(ctx context.Context, key string) error {
    return inv.client.Do(ctx,
        inv.client.B().Publish().Channel(invalidationChannel).Message(key).Build(),
    ).Error()
}

// Started as a goroutine on server startup
func (inv *Invalidator) Subscribe(ctx context.Context) {
    inv.client.Receive(ctx,
        inv.client.B().Subscribe().Channel(invalidationChannel).Build(),
        func(msg valkey.PubSubMessage) {
            inv.cache.Invalidate(msg.Message)
        },
    )
}
```

#### OCI skill storage (oras-go)

```go
// internal/store/oci.go

import "oras.land/oras-go/v2"

const skillMediaType = "application/vnd.skillctl.skill.v1.tar+gzip"

func (s *OCIStore) PushSkill(ctx context.Context, name, version string, archive []byte) (digest string, err error) {
    ref := fmt.Sprintf("%s/skills/%s:%s", s.registry, name, version)
    // Build OCI manifest with skillMediaType layer
    // Push using oras.Copy
    // Return content digest
}

func (s *OCIStore) GeneratePullToken(ctx context.Context, name, version string) (token string, err error) {
    // For ghcr.io: exchange service account credentials for a short-lived
    // registry token scoped to pull on the specific repository.
    // TTL: 15 minutes.
    // Return the token — CLI uses it as Bearer auth when pulling.
}
```

#### Dockerfile

Identical to v1 (multi-stage, scratch final image, CGO_ENABLED=0, Trivy scan stage).

#### Blue/Green deployment

Same pattern as v1 — deploy with `--set image.tag=sha-<sha>` as a second Helm release (or ArgoCD preview), run smoke suite, then update the primary release. In a Nebari/ArgoCD setup this is handled via the org repo: PR updates the image tag, ArgoCD syncs to a canary Application, e2e pass triggers a merge that updates the production Application.

### 3.5 Phase 3 — Helm Chart

#### `chart/Chart.yaml`

```yaml
apiVersion: v2
name: skillctl
description: A Kubernetes-native registry for Claude Code skills
type: application
version: 0.1.0      # chart version, bumped independently of app version
appVersion: 0.1.0   # app version, set by CI from git tag
dependencies:
  # CloudNativePG is a cluster-level operator — we don't bundle it.
  # Document as a prerequisite. The chart creates a Cluster CR, not the operator.
  # No subchart dependencies — keeps the chart portable.
```

#### `chart/values.yaml` (key sections)

```yaml
image:
  repository: ghcr.io/yourorg/skillctl-backend
  tag: ""           # defaults to .Chart.AppVersion
  pullPolicy: IfNotPresent

replicaCount: 2

resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    memory: 256Mi

service:
  type: ClusterIP
  port: 8080

# PostgreSQL — uses CloudNativePG Cluster CR
postgres:
  # If enabled, chart creates a CloudNativePG Cluster in the same namespace.
  # Set enabled=false and provide externalDatabaseURL if using an external DB.
  enabled: true
  instances: 1        # bump to 3 for HA
  storage:
    size: 5Gi
    storageClass: ""  # leave empty for default
  externalDatabaseURL: ""  # used when enabled=false

# Valkey
valkey:
  enabled: true
  storage:
    size: 1Gi
    storageClass: ""
  externalURL: ""  # used when enabled=false

# OCI registry for skill archives
oci:
  registry: ghcr.io
  repository: ""    # e.g. yourorg/skills  — REQUIRED, no default
  # Credentials for the server to push/pull skill archives.
  # Provide as a pre-created K8s secret name, or set credentials directly.
  credentialsSecret: ""
  username: ""
  password: ""      # use existingSecret in production

# Auth — generic OIDC, no provider-specific defaults
auth:
  oidc:
    issuerURL: ""   # REQUIRED — e.g. https://keycloak.example.com/realms/myrealm
    clientID: ""    # REQUIRED
    allowedDomain: ""    # optional — restrict to email domain
    adminGroup: "skillctl-admins"  # OIDC groups claim value for admins
  pushTokens: {}    # map of token→team for write auth, stored in a K8s Secret

# Rate limiting (env vars passed to server)
rateLimit:
  readPerIPPerMin: 100
  writePerTokenPerMin: 10
  adminPerEmailPerMin: 30

# Federation poller
federation:
  pollIntervalSeconds: 60

# Nebari integration — opt-in
nebari:
  enabled: false
  app:
    displayName: "skillctl"
    subdomain: "skillctl"    # results in skillctl.{nebari-domain}
    icon: ""                 # optional URL to icon
    description: "Claude Code skill registry"
    # Keycloak client ID for the Nebari OIDC SecurityPolicy.
    # This is the client the nebari-operator creates in Keycloak.
    keycloakClientID: ""     # REQUIRED when nebari.enabled=true

ingress:
  # Used when nebari.enabled=false and you want a plain ingress
  enabled: false
  className: ""
  host: ""
  tls: []
```

#### `chart/templates/nebariapp.yaml`

```yaml
{{- if .Values.nebari.enabled }}
apiVersion: nebari.dev/v1alpha1
kind: NebariApp
metadata:
  name: {{ include "skillctl.fullname" . }}
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "skillctl.labels" . | nindent 4 }}
spec:
  displayName: {{ .Values.nebari.app.displayName | quote }}
  subdomain: {{ .Values.nebari.app.subdomain | quote }}
  description: {{ .Values.nebari.app.description | quote }}
  {{- if .Values.nebari.app.icon }}
  icon: {{ .Values.nebari.app.icon | quote }}
  {{- end }}
  service:
    name: {{ include "skillctl.fullname" . }}
    port: {{ .Values.service.port }}
  auth:
    keycloakClientID: {{ required "nebari.app.keycloakClientID is required when nebari.enabled=true" .Values.nebari.app.keycloakClientID | quote }}
{{- end }}
```

#### `chart/crds/nebariapp-crd.yaml`

The NebariApp CRD is copied from a pinned nebari-operator release and committed to the chart's `crds/` directory. Helm installs CRDs from `crds/` automatically on first install and never deletes them on uninstall (safe behavior). The CRD is harmless on non-Nebari clusters — it just sits there unused unless `nebari.enabled=true` actually creates a NebariApp resource.

Pin to a specific nebari-operator version and document the update procedure in the chart's CHANGELOG.

#### Publishing the chart

The `ci-chart.yml` workflow:

```
On PR:
└── helm lint chart/
└── helm template chart/ -f chart/ci/test-values.yaml (dry-run render check)
└── chart-testing (ct lint) — validates against values.schema.json

On merge to main / tag:
└── helm package chart/ --app-version $TAG --version $TAG
└── helm push skillctl-$TAG.tgz oci://ghcr.io/yourorg/charts
```

Install command for users:
```bash
helm install skillctl oci://ghcr.io/yourorg/charts/skillctl \
  --version 0.1.0 \
  --namespace skillctl --create-namespace \
  -f my-values.yaml
```

### 3.6 Phase 4 — CLI

#### Config file: `~/.config/skillctl/config.yaml`

```yaml
api_url: https://skillctl.example.com
skills_dir: ~/.claude/skills

auth:
  oidc_issuer: https://keycloak.example.com/realms/myrealm
  client_id: skillctl-cli
  # token cached at ~/.config/skillctl/credentials.json after login
```

The `auth.oidc_issuer` and `auth.client_id` values are set once during onboarding (`skillctl config set auth.oidc_issuer <url>`). The org documents the correct values in their internal README.

Optionally, the org can build a pre-configured binary via GoReleaser ldflags:

```yaml
# in .goreleaser.yml, for an org-specific build
ldflags:
  - -X main.defaultOIDCIssuer=https://keycloak.example.com/realms/myrealm
  - -X main.defaultClientID=skillctl-cli
  - -X main.defaultAPIURL=https://skillctl.openteams.com
```

This means new devs can run `skillctl auth login` immediately without any config step.

#### Generic OIDC device flow

```go
// cli/internal/auth/device_flow.go

type OIDCConfig struct {
    IssuerURL string
    ClientID  string
}

func (c *OIDCConfig) Login(ctx context.Context) error {
    // 1. Fetch {IssuerURL}/.well-known/openid-configuration
    //    to discover device_authorization_endpoint and token_endpoint
    meta, err := fetchOIDCMetadata(ctx, c.IssuerURL)

    // 2. POST to device_authorization_endpoint with client_id + scope
    dc, err := requestDeviceCode(ctx, meta.DeviceAuthorizationEndpoint, c.ClientID)

    // 3. Print user_code + verification_uri
    fmt.Printf("\nOpen: %s\nEnter code: %s\n\n", dc.VerificationURI, dc.UserCode)

    // 4. Poll token_endpoint until user completes or timeout
    tok, err := pollForToken(ctx, meta.TokenEndpoint, c.ClientID, dc)

    // 5. Save to ~/.config/skillctl/credentials.json
    return saveToken(tok)
}
```

Works with Keycloak out of the box (Keycloak supports RFC 8628 device flow). Also works with any other standard OIDC provider.

#### OCI pull in CLI

```go
// cli/internal/oci/pull.go

import "oras.land/oras-go/v2"

func PullSkill(ctx context.Context, ociRef, pullToken, destDir string) error {
    // Configure oras remote with Bearer token auth
    repo, err := remote.NewRepository(ociRef)
    repo.Client = &auth.Client{
        Credential: auth.StaticCredential("", auth.Credential{
            AccessToken: pullToken,
        }),
    }
    // Pull the skill layer (application/vnd.skillctl.skill.v1.tar+gzip)
    // Extract to destDir
}
```

#### CLI commands

All commands from v1 are unchanged in behavior. The differences are:
- `skillctl auth login` uses OIDC device flow against the configured issuer instead of Google OAuth
- `skillctl install` pulls from OCI registry via oras instead of a GCS signed URL
- `skillctl config set auth.oidc_issuer <url>` and `skillctl config set auth.client_id <id>` are new subcommands

### 3.7 Phase 5 — CI/CD Pipelines

#### CLI pipeline (`ci-cli.yml`) — same as v1

```
Stage 1: Quality  → gitleaks, golangci-lint, buf-check
Stage 2: Unit tests
Stage 3: Build matrix (5 targets, parallel)
Stage 4: E2E tests (parallel per platform)
Stage 5: Release via GoReleaser (on tag)
Stage 6: Docs to GitHub Pages
```

#### Backend pipeline (`ci-backend.yml`) — same as v1 with one change

The e2e stage deploys using Helm against a kind cluster (not Cloud Run), making the e2e stage fully self-contained in CI without needing a real cloud environment:

```
Stage 1: Quality
Stage 2: Unit tests
Stage 3: Container build → Trivy scan → smoke test → push sha-tagged image

Stage 4: E2E + DAST
├── Spin up kind cluster
├── Install CloudNativePG operator
├── helm install skillctl ./chart -f chart/ci/test-values.yaml
│     (test-values.yaml uses in-cluster Valkey, disables Nebari)
├── Wait for rollout
├── Run e2e/backend/ suite + marketplace tests
├── OWASP ZAP baseline scan
└── Tear down kind cluster

Stage 5: Release (merge to main)
└── goreleaser → ghcr.io :latest + :vX.Y.Z
```

#### Chart pipeline (`ci-chart.yml`)

```
On PR (paths: chart/**):
├── helm lint chart/
├── helm template (dry-run render)
├── ct lint (validates values.schema.json)
└── kind-based install test (helm install + helm test)

On tag vX.Y.Z:
├── helm package --version $TAG --app-version $TAG
└── helm push oci://ghcr.io/yourorg/charts/skillctl:$TAG
```

#### Proto pipeline (`ci-proto.yml`) — same as v1

#### Infrastructure: none in this repo

No OpenTofu in `skillctl`. All infra lives in the org GitOps repo. The `skillctl` repo is purely application code.

#### GitHub Actions Required Secrets (skillctl repo)

| Secret | Description |
|---|---|
| `GHCR_TOKEN` | Push containers + Helm chart to ghcr.io |
| `SCOOP_BUCKET_TOKEN` | PAT for scoop-bucket repo |
| `SKILLCTL_TEST_OIDC_TOKEN` | Pre-minted OIDC token for a test user (e2e) |
| `SKILLCTL_TEST_ADMIN_TOKEN` | Pre-minted OIDC token for a test admin (e2e marketplace tests) |
| `SKILLCTL_TEST_PUSH_TOKEN` | Push token for e2e publish tests |
| `SKILLCTL_TEST_OCI_PASSWORD` | Registry credentials for e2e kind cluster |

---

## 4. Repo 2: `skillctl-deploy` (Org GitOps)

This repo is internal-only. It contains everything org-specific.

### 4.1 Repository Structure

```
skillctl-deploy/
├── argocd/
│   ├── dev/
│   │   └── application.yaml     # ArgoCD Application for dev namespace
│   └── prod/
│       └── application.yaml     # ArgoCD Application for prod namespace
│
├── helm-values/
│   ├── base/
│   │   └── values.yaml          # Shared values (auth config, OCI registry, etc.)
│   ├── dev/
│   │   └── values.yaml          # Dev overrides (lower replicas, scale-to-zero)
│   └── prod/
│       └── values.yaml          # Prod overrides (HA postgres, HPA settings)
│
├── keycloak/
│   └── skillctl-client.yaml     # Keycloak client config (realm export fragment)
│                                # Defines the skillctl-cli client with device flow
│                                # and the skillctl-admins group mapping
│
├── scripts/
│   └── seed-marketplaces.sh     # One-time script: adds Anthropic + Superpowers
│
└── README.md
```

### 4.2 ArgoCD Application Manifests

```yaml
# argocd/prod/application.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: skillctl-prod
  namespace: argocd
spec:
  project: default
  source:
    repoURL: ghcr.io/yourorg/charts
    chart: skillctl
    targetRevision: 0.3.1          # pin to a specific chart version
    helm:
      valueFiles:
        - $values/helm-values/base/values.yaml
        - $values/helm-values/prod/values.yaml
  sources:
    - repoURL: https://github.com/yourorg/skillctl-deploy
      targetRevision: main
      ref: values
    - repoURL: ghcr.io/yourorg/charts
      chart: skillctl
      targetRevision: 0.3.1
      helm:
        valueFiles:
          - $values/helm-values/base/values.yaml
          - $values/helm-values/prod/values.yaml
  destination:
    server: https://kubernetes.default.svc
    namespace: skillctl-prod
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
```

### 4.3 Org Values Overlay

```yaml
# helm-values/base/values.yaml — shared across dev + prod

auth:
  oidc:
    issuerURL: https://keycloak.openteams.com/realms/openteams
    clientID: skillctl-cli
    allowedDomain: openteams.com
    adminGroup: skillctl-admins

oci:
  registry: ghcr.io
  repository: openteams/skills
  credentialsSecret: skillctl-oci-credentials

nebari:
  enabled: true
  app:
    displayName: "skillctl"
    subdomain: "skillctl"
    description: "Internal Claude Code skill registry"
    keycloakClientID: skillctl-server
```

```yaml
# helm-values/prod/values.yaml

replicaCount: 3

postgres:
  instances: 3
  storage:
    size: 20Gi
    storageClass: ssd

valkey:
  storage:
    size: 2Gi
    storageClass: ssd

image:
  tag: "0.3.1"    # bumped by automated PR when new skillctl release is tagged
```

### 4.4 NebariApp CRD

The NebariApp CRD is installed by Helm from the `chart/crds/` directory in the skillctl chart. No separate step needed. The `nebari.enabled=true` value in base values.yaml activates the NebariApp resource, which the nebari-operator picks up to configure:

- An HTTPRoute on the shared Envoy Gateway pointing at the skillctl Service
- A cert-manager certificate for `skillctl.{nebari-domain}`
- A Keycloak OIDC SecurityPolicy (Envoy ExtAuthz) on the route

This means the skillctl server itself does not handle TLS termination — Envoy Gateway does. The server receives pre-authenticated requests with the OIDC token forwarded as a header. The server still validates the token independently (defense in depth — the server should not trust Envoy blindly).

### 4.5 Keycloak Client Configuration

```yaml
# keycloak/skillctl-client.yaml
# Import this into your Keycloak realm via the admin console or Terraform Keycloak provider

clients:
  # CLI client — device flow, public client (no secret)
  - clientId: skillctl-cli
    publicClient: true
    standardFlowEnabled: false
    directAccessGrantsEnabled: false
    deviceAuthorizationGrantEnabled: true
    defaultScopes: [openid, email, profile, groups]

  # Server client — used by nebari-operator for OIDC SecurityPolicy
  - clientId: skillctl-server
    publicClient: false
    # clientSecret: managed by nebari-operator

groups:
  - name: skillctl-admins
    # Add admin users here

# Protocol mapper to include groups in token
protocolMappers:
  - name: groups
    protocol: openid-connect
    protocolMapper: oidc-group-membership-mapper
    config:
      claim.name: groups
      full.path: "false"
      access.token.claim: "true"
      id.token.claim: "true"
```

---

## 5. Federation & Marketplace Management

Identical to v1 Phase 6 with these storage changes:

- All marketplace state (marketplaces table, federated_skills table, audit_log) is in PostgreSQL instead of Firestore
- The distributed lock for the federation poller uses PostgreSQL advisory locks (`pg_try_advisory_lock`) instead of a Firestore document
- The real-time cache update uses Valkey pub/sub instead of Firestore snapshot listener

```go
// internal/federation/poller.go

func (p *Poller) acquireLock(ctx context.Context, db *pgx.Conn) (bool, error) {
    // PostgreSQL advisory lock — only one instance runs the sync at a time
    // Lock key: hash of "federation-poller" (a fixed int64)
    var acquired bool
    err := db.QueryRow(ctx,
        "SELECT pg_try_advisory_lock($1)", federationLockKey).Scan(&acquired)
    return acquired, err
}
```

Pre-seeded marketplaces via `scripts/seed-marketplaces.sh` — same as v1.

---

## 6. Auth Design

Full auth flow for reference:

```
1. Dev runs: skillctl auth login
   → CLI fetches OIDC discovery doc from issuerURL
   → CLI POSTs to device_authorization_endpoint
   → CLI prints user_code + verification_uri
   → Dev opens browser, logs into Keycloak, enters code
   → CLI polls token_endpoint, receives access_token + refresh_token
   → Tokens saved to ~/.config/skillctl/credentials.json

2. Dev runs: skillctl explore
   → CLI reads access_token from credentials.json
   → If expired: CLI uses refresh_token to get new access_token (transparent)
   → CLI sends: Authorization: Bearer <access_token>
   → Server validates token: signature (JWKS), expiry, audience (clientID), domain
   → Server serves request

3. Dev runs: skillctl push
   → Same token flow
   → Server additionally checks X-Push-Token header against push_tokens config

4. Admin runs: skillctl marketplace add
   → Same token flow
   → Server checks groups claim in token for adminGroup value
   → No external API call — group membership is in the token

5. [If Nebari] Browser access to skillctl web UI (future)
   → Request hits Envoy Gateway
   → No OIDC session → redirect to Keycloak
   → Keycloak auth → token set as cookie
   → Envoy forwards request with token header to skillctl-server
   → skillctl-server validates independently (defense in depth)
```

---

## 7. Dogfood Skill

Same as v1 — `skills/goreleaser/SKILL.md` in the skillctl repo, pushed to the registry on first deploy via `scripts/seed-marketplaces.sh`.

---

## 8. Outstanding Decisions & Constraints

1. **Keycloak client setup** — The `keycloak/skillctl-client.yaml` file in the org repo documents the required config, but someone must apply it. Options: manual import via Keycloak admin console, the Terraform Keycloak provider (if your Nebari deployment uses it), or a one-time `kcadm.sh` script. Decide which is canonical for your org before deployment.

2. **CloudNativePG operator** — Must be installed in the cluster before deploying the skillctl chart. The chart creates a CloudNativePG `Cluster` CR but does not install the operator. Document this as a prerequisite in the org README. If it's not already in your Nebari cluster, add it as a separate ArgoCD Application in the skillctl-deploy repo.

3. **OCI registry credentials** — The skillctl server needs push/pull access to `ghcr.io/openteams/skills`. Create a GitHub machine account or use a GitHub App for this. Store credentials as a K8s secret (`skillctl-oci-credentials`) created out-of-band (not in the GitOps repo). Reference the secret name in values.yaml.

4. **Image tag automation** — When a new skillctl release is tagged, the prod values.yaml needs `image.tag` bumped. Automate this with a GitHub Action in the skillctl-deploy repo that opens a PR on new skillctl releases (listen for release events from the skillctl repo via a workflow_dispatch or repository_dispatch trigger).

5. **Nebari CRD version pinning** — The `chart/crds/nebariapp-crd.yaml` must be kept in sync with the nebari-operator version running in your cluster. Add a note in `chart/CHANGELOG.md` whenever it is updated. Mismatched CRD versions are a common silent failure mode.

6. **OIDC `groups` claim in Keycloak** — Keycloak does not include group membership in tokens by default. The protocol mapper in `keycloak/skillctl-client.yaml` must be applied for admin checks to work. Without it, `IsAdmin` always returns false and `skillctl marketplace` commands return PermissionDenied for everyone.

7. **Pre-configured binary for onboarding** — The org can build a custom binary via GoReleaser ldflags that pre-sets `oidcIssuer`, `clientID`, and `apiURL`. This removes the manual `skillctl config set` step for new devs. Consider hosting the org-specific binary as a private GitHub Release in the skillctl-deploy repo, or distributing via an internal Homebrew tap / Scoop bucket that overrides the public one.

8. **Windows self-update** — Use `inconshreveable/go-update` for the locked-binary swap. Same constraint as v1.

9. **PostgreSQL backup** — CloudNativePG supports WAL archiving to S3-compatible storage. Configure this in the prod values overlay. The dev environment can skip it. Define backup retention policy before going to production.

10. **Valkey persistence** — Valkey is used for pub/sub only, not as a durable store, so AOF/RDB persistence is not required. Configure Valkey with `--save ""` (no persistence) to reduce I/O and storage overhead. If Valkey restarts, the worst case is that cache entries go stale until the next write invalidates them — which is acceptable.

11. **Proto versioning + VERSION file** — Same as v1. Maintain `VERSION` file in repo root. Buf breaking-change check reads it.

12. **Generated code drift** — Same as v1. `buf generate && git diff --exit-code gen/` in CI. Add `gen/** linguist-generated=true` to `.gitattributes`.

13. **Scoop bucket** — Requires a separate public GitHub repo `yourorg/scoop-bucket`. Create before first GoReleaser run.

14. **chart-testing (ct) prerequisites** — The `ci-chart.yml` pipeline uses `helm/chart-testing-action`. It requires a `chart/ci/test-values.yaml` that provides all required values (oci.repository, auth.oidc.issuerURL, auth.oidc.clientID) with test-safe values pointing at the kind cluster's local services.

---

*End of work plan v2. Implementation order: Proto (Phase 1) → Backend (Phase 2) → Chart (Phase 3) → CLI (Phase 4) → CI/CD (Phase 5) → Federation (§5) → Org repo (§4) → Dogfood (§7). The org repo (§4) cannot be completed until the chart is published from Phase 3.*
