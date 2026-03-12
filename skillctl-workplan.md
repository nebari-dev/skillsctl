# skillctl — Complete Work Plan

**For:** Claude Code  
**Project:** A CLI tool + backend registry for discovering, installing, and publishing Claude Code skills, with federated marketplace support and admin-controlled external skill whitelisting  
**Last updated:** 2026-03-11

---

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [Repository Structure](#2-repository-structure)
3. [Technology Decisions](#3-technology-decisions)
4. [Phase 1 — Protobuf & ConnectRPC Setup](#4-phase-1--protobuf--connectrpc-setup)
5. [Phase 2 — Backend Service](#5-phase-2--backend-service)
6. [Phase 3 — CLI](#6-phase-3--cli)
7. [Phase 4 — Infrastructure (OpenTofu)](#7-phase-4--infrastructure-opentofu)
8. [Phase 5 — CI/CD Pipelines](#8-phase-5--cicd-pipelines)
9. [Phase 6 — Federation & Marketplace Management](#9-phase-6--federation--marketplace-management)
10. [Phase 7 — Dogfood Skill](#10-phase-7--dogfood-skill)
11. [Secrets & Configuration Reference](#11-secrets--configuration-reference)
12. [Outstanding Decisions & Constraints](#12-outstanding-decisions--constraints)

---

## 1. Architecture Overview

```
┌──────────────────────────────────────────────────────────────────────┐
│                            GCP Project                               │
│                                                                      │
│  Cloud Run (skillctl-api)                                            │
│    ├── ConnectRPC server (Go)                                        │
│    ├── In-memory skill cache (warm from Firestore on startup)        │
│    ├── Firestore real-time listener (keeps cache fresh)              │
│    └── Federation poller (syncs whitelisted external marketplaces)   │
│                                                                      │
│  Firestore (native mode, us-central1)                                │
│    ├── skills/{name}/versions/{semver}       (internal skills)       │
│    ├── marketplaces/{id}                     (whitelist registry)    │
│    └── federated_skills/{marketplace}/{name} (cached external skills)│
│                                                                      │
│  Cloud Storage                                                       │
│    ├── gs://skillctl-skills-{env}/   (internal skill .tar.gz)        │
│    └── gs://skillctl-tfstate/        (OpenTofu state)                │
│                                                                      │
│  Secret Manager                                                       │
│    └── push-api-tokens (per-team static tokens for writes)           │
└──────────────────────────────────────────────────────────────────────┘
         │  HTTPS / ConnectRPC (JSON or binary)
┌────────┴──────────────────────────────────────────────────────────┐
│  skillctl (Go CLI binary)                                          │
│    Platform targets:                                               │
│      linux/amd64, linux/arm64                                      │
│      darwin/amd64, darwin/arm64                                    │
│      windows/amd64 (.exe)                                          │
│                                                                    │
│  Auth: Google OAuth device flow                                    │
│    Read endpoints      → Google ID token (openteams.com only)      │
│    Write endpoints     → Google ID token + team push token         │
│    Admin endpoints     → Google ID token + admin role claim        │
└────────────────────────────────────────────────────────────────────┘
         │  Federated fetch (server-side, scheduled poll)
┌────────┴──────────────────────────────────────────────────────────┐
│  External Marketplaces (whitelisted by admins)                     │
│    e.g. github.com/anthropics/skills                               │
│         github.com/obra/superpowers                                │
│    Each marketplace entry specifies:                               │
│      - source URL + format (GitHub repo, agentskills.io API, etc.) │
│      - sync frequency                                              │
│      - optional skill-level allowlist (specific skills within a    │
│        marketplace, rather than the whole thing)                   │
└────────────────────────────────────────────────────────────────────┘
```

### Key Design Decision: Server-Side Federation

External marketplace syncing happens **server-side on a schedule**, not client-side on demand. This is the critical architectural choice that makes the whitelist enforceable:

- Admins whitelist a marketplace once. The server polls it periodically and caches approved skills in Firestore.
- Devs never talk directly to external marketplaces — they only ever talk to `skillctl-api`. The whitelist cannot be bypassed by a determined dev pointing their client elsewhere, because install downloads are proxied or re-signed through the backend.
- If a marketplace is removed from the whitelist, its skills immediately disappear from `skillctl explore` and cannot be installed. No client-side config changes required.

### Environments

Two environments: **dev** and **prod**. Each has its own:
- GCP project (`skillctl-dev`, `skillctl-prod`)
- Cloud Run service
- Firestore database
- GCS buckets

Dev deploys on every merge to `main`. Prod deploys after blue/green validation succeeds in dev.

---

## 2. Repository Structure

```
skillctl/
├── .github/
│   ├── workflows/
│   │   ├── ci-cli.yml           # CLI pipeline (lint → test → build → e2e → release)
│   │   ├── ci-backend.yml       # Backend pipeline (lint → test → build → e2e → push image)
│   │   ├── ci-proto.yml         # Buf lint + breaking check
│   │   ├── ci-infra.yml         # OpenTofu plan (PR comment) + apply (main)
│   │   └── ci-docs.yml          # Generate + publish CLI docs to GitHub Pages
│   └── PULL_REQUEST_TEMPLATE.md
│
├── proto/
│   ├── buf.yaml
│   ├── buf.gen.yaml             # Generates Go + Python clients
│   └── skillctl/v1/
│       ├── skill.proto
│       ├── registry.proto       # Service definition (internal + federated skills)
│       ├── federation.proto     # Marketplace whitelist admin RPCs
│       └── auth.proto
│
├── gen/                         # Generated code — committed, not hand-edited
│   ├── go/skillctl/v1/          # Go generated types + ConnectRPC stubs
│   └── python/skillctl/v1/      # Python generated types + ConnectRPC stubs
│
├── backend/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── auth/
│   │   │   ├── middleware.go    # Google ID token validation + domain + admin role
│   │   │   └── token.go        # Push token validation against Secret Manager
│   │   ├── registry/
│   │   │   ├── service.go       # ConnectRPC handler implementations
│   │   │   ├── cache.go         # In-memory cache + Firestore listener (internal + federated)
│   │   │   └── search.go        # Full-text search across internal + federated skills
│   │   ├── federation/
│   │   │   ├── service.go       # Admin RPCs: add/remove/list/sync marketplaces
│   │   │   ├── poller.go        # Background goroutine: syncs whitelisted marketplaces
│   │   │   ├── github.go        # Fetcher for GitHub-hosted skill repos
│   │   │   └── agentskills.go   # Fetcher for agentskills.io API
│   │   ├── store/
│   │   │   ├── firestore.go     # Firestore reads/writes (skills + marketplaces)
│   │   │   └── gcs.go           # GCS upload + signed URL generation
│   │   └── ratelimit/
│   │       └── middleware.go    # Token-bucket rate limiting (per IP + per push token)
│   ├── Dockerfile               # Multi-stage → scratch final image
│   └── .goreleaser-backend.yml
│
├── cli/
│   ├── main.go
│   ├── cmd/
│   │   ├── root.go              # Cobra root, Viper config binding
│   │   ├── explore.go           # skillctl explore [--source internal|external|all]
│   │   ├── install.go           # skillctl install <name>[@version] [--from <marketplace>]
│   │   ├── push.go              # skillctl push <path> [--draft]
│   │   ├── auth.go              # skillctl auth login / logout / status
│   │   ├── update.go            # skillctl update (self-update)
│   │   ├── marketplace.go       # skillctl marketplace (admin subcommands)
│   │   └── docs.go              # skillctl docs (generate markdown — CI only)
│   ├── internal/
│   │   ├── api/
│   │   │   └── client.go        # Typed ConnectRPC client wrapper
│   │   ├── auth/
│   │   │   ├── device_flow.go   # Google OAuth device flow
│   │   │   └── token_store.go   # Persist token to ~/.config/skillctl/
│   │   ├── skill/
│   │   │   ├── package.go       # tar.gz pack/unpack
│   │   │   └── validate.go      # SKILL.md frontmatter validation
│   │   ├── selfupdate/
│   │   │   └── update.go        # Check GitHub releases, download + replace binary
│   │   └── config/
│   │       └── config.go        # Viper config loading
│   └── .goreleaser.yml
│
├── infra/
│   ├── modules/
│   │   ├── cloudrun/
│   │   ├── firestore/           # Includes marketplace + federated_skills collections
│   │   ├── gcs/
│   │   └── iam/
│   ├── envs/
│   │   ├── dev/
│   │   │   ├── main.tf
│   │   │   ├── variables.tf
│   │   │   └── terraform.tfvars
│   │   └── prod/
│   │       ├── main.tf
│   │       ├── variables.tf
│   │       └── terraform.tfvars
│   └── backend.tf
│
├── e2e/
│   ├── cli/
│   │   ├── explore_test.go
│   │   ├── install_test.go
│   │   ├── push_test.go
│   │   └── marketplace_test.go  # Federation admin + federated install e2e
│   └── backend/
│       └── zap-baseline.yaml
│
├── docs/
│   └── .gitkeep
│
├── skills/
│   └── goreleaser/
│       └── SKILL.md
│
├── Makefile
├── .golangci.yml
├── .gitleaks.toml
└── README.md
```

---

## 3. Technology Decisions

| Concern | Choice | Rationale |
|---|---|---|
| RPC framework | **ConnectRPC** | Same `.proto` source as gRPC; speaks gRPC, gRPC-Web, and JSON/HTTP from same server. Works with `curl`. DAST tools (ZAP) can hit JSON endpoints. |
| CLI framework | **Cobra + Viper** | Standard Go CLI stack. Viper handles config file + env var layering automatically. |
| Auth (read) | **Google ID token, device flow** | No browser required on headless/remote machines. Token cached locally. |
| Auth (write) | **ID token + team push token** | Push token stored in GCP Secret Manager, distributed per team via Slack. |
| Auth (admin) | **ID token + `skillctl-admins` Google Group membership** | Admin role checked against Google Groups API at runtime. No separate token required — group membership is the gate. |
| Domain restriction | **openteams.com** | Backend validates `hd` claim in Google ID token on every request. |
| Federation model | **Server-side poll, server-enforced whitelist** | Clients never talk to external marketplaces directly. Whitelist cannot be bypassed client-side. External skills cached in Firestore, available offline from external sources. |
| External skill install | **Proxy through backend** | For whitelisted external skills, the backend fetches the archive from the external source, validates it, and serves a short-lived signed URL. This keeps the whitelist enforceable at download time, not just at discovery time. |
| Search | **In-memory + Firestore real-time listener** | Covers both internal and federated skill caches. |
| Rate limiting | **Token-bucket, server-side** | Per-IP for read, per-push-token for writes. CLI also backs off on 429. |
| Container | **Scratch final image** | `CGO_ENABLED=0` static binary. Must be validated with a container smoke test in CI. |
| State backend | **GCS bucket `skillctl-tfstate`** | Separate from app buckets. State locking via GCS native object versioning. |
| Windows distribution | **Scoop** | Avoids Windows Defender SmartScreen on unsigned `.exe`. GoReleaser publishes Scoop manifest. |
| Secrets scanning | **Gitleaks** | Runs as pre-commit hook + CI stage. |
| DAST | **OWASP ZAP** (baseline scan) | Targets the running dev Cloud Run service post-deploy. |
| Proto breaking changes | **buf breaking — warn pre-1.0, block post-1.0** | CI fails on breaking changes if `version >= 1.0.0`. |
| Release automation | **GoReleaser** | Cross-compilation, GitHub Release, Scoop manifest, container push. |
| Docs | **cobra/doc → GitHub Pages** | `skillctl docs` generates markdown; CI commits to `gh-pages` branch. |

---

## 4. Phase 1 — Protobuf & ConnectRPC Setup

### 4.1 `buf.yaml`

```yaml
version: v2
modules:
  - path: proto
deps:
  - buf.build/googleapis/googleapis
breaking:
  use:
    - FILE
lint:
  use:
    - DEFAULT
```

### 4.2 `buf.gen.yaml`

```yaml
version: v2
plugins:
  - plugin: buf.build/protocolbuffers/go
    out: gen/go
    opt:
      - paths=source_relative

  - plugin: buf.build/connectrpc/go
    out: gen/go
    opt:
      - paths=source_relative

  - plugin: buf.build/protocolbuffers/python
    out: gen/python

  - plugin: buf.build/connectrpc/python
    out: gen/python
```

### 4.3 Proto Service Definitions

#### `proto/skillctl/v1/registry.proto`

```protobuf
syntax = "proto3";
package skillctl.v1;

import "google/protobuf/timestamp.proto";

service RegistryService {
  // Read — requires Google ID token (openteams.com)
  rpc ListSkills(ListSkillsRequest) returns (ListSkillsResponse);
  rpc GetSkill(GetSkillRequest) returns (GetSkillResponse);
  rpc SearchSkills(SearchSkillsRequest) returns (SearchSkillsResponse);
  rpc GetDownloadURL(GetDownloadURLRequest) returns (GetDownloadURLResponse);

  // Write — requires ID token + push token header
  rpc PublishSkill(PublishSkillRequest) returns (PublishSkillResponse);
  rpc RecordInstall(RecordInstallRequest) returns (RecordInstallResponse);

  // Meta
  rpc GetLatestCLIVersion(GetLatestCLIVersionRequest) returns (GetLatestCLIVersionResponse);
}

message Skill {
  string name = 1;
  string description = 2;
  string owner = 3;
  repeated string tags = 4;
  string latest_version = 5;
  int64 install_count = 6;
  google.protobuf.Timestamp created_at = 7;
  google.protobuf.Timestamp updated_at = 8;
  SkillSource source = 9;       // INTERNAL or FEDERATED
  string marketplace_id = 10;   // populated for FEDERATED skills
  string upstream_url = 11;     // original URL in external marketplace
}

enum SkillSource {
  SKILL_SOURCE_UNSPECIFIED = 0;
  SKILL_SOURCE_INTERNAL = 1;
  SKILL_SOURCE_FEDERATED = 2;
}

message ListSkillsRequest {
  repeated string tags = 1;
  SkillSource source_filter = 2;  // default: ALL
  string marketplace_id = 3;      // filter by specific marketplace
  int32 page_size = 4;
  string page_token = 5;
}

// ... (remaining request/response messages)
```

#### `proto/skillctl/v1/federation.proto`

```protobuf
syntax = "proto3";
package skillctl.v1;

import "google/protobuf/timestamp.proto";

// Admin-only — requires Google ID token with admin group membership
service FederationService {
  rpc AddMarketplace(AddMarketplaceRequest) returns (AddMarketplaceResponse);
  rpc RemoveMarketplace(RemoveMarketplaceRequest) returns (RemoveMarketplaceResponse);
  rpc ListMarketplaces(ListMarketplacesRequest) returns (ListMarketplacesResponse);
  rpc UpdateMarketplace(UpdateMarketplaceRequest) returns (UpdateMarketplaceResponse);
  rpc TriggerSync(TriggerSyncRequest) returns (TriggerSyncResponse);

  // Skill-level allowlist within a marketplace
  rpc AllowSkill(AllowSkillRequest) returns (AllowSkillResponse);
  rpc BlockSkill(BlockSkillRequest) returns (BlockSkillResponse);
  rpc ListAllowedSkills(ListAllowedSkillsRequest) returns (ListAllowedSkillsResponse);
}

message Marketplace {
  string id = 1;                      // stable identifier, e.g. "anthropic-official"
  string display_name = 2;            // e.g. "Anthropic Official Skills"
  string source_url = 3;              // e.g. "https://github.com/anthropics/skills"
  MarketplaceType type = 4;
  MarketplaceMode mode = 5;           // ALLOWLIST_ALL or ALLOWLIST_SPECIFIC
  repeated string allowed_skills = 6; // only used when mode = ALLOWLIST_SPECIFIC
  int32 sync_interval_minutes = 7;    // how often to poll
  google.protobuf.Timestamp last_synced_at = 8;
  int32 skill_count = 9;
  bool enabled = 10;
  string added_by = 11;               // email of admin who added it
  google.protobuf.Timestamp added_at = 12;
}

enum MarketplaceType {
  MARKETPLACE_TYPE_UNSPECIFIED = 0;
  MARKETPLACE_TYPE_GITHUB_REPO = 1;   // GitHub repo with SKILL.md files
  MARKETPLACE_TYPE_AGENTSKILLS_IO = 2; // agentskills.io API
}

enum MarketplaceMode {
  MARKETPLACE_MODE_UNSPECIFIED = 0;
  MARKETPLACE_MODE_ALL = 1;           // whitelist the entire marketplace
  MARKETPLACE_MODE_SPECIFIC = 2;      // only allowed_skills are synced
}

// ... (request/response messages)
```

### 4.4 Breaking Change Policy

In `ci-proto.yml`:
- Always run `buf lint`
- Run `buf breaking --against "https://github.com/yourorg/skillctl.git#branch=main"`
- Parse `VERSION` file: if `>= 1.0.0` fail the job; if `< 1.0.0` post a PR warning comment only
- Run `buf generate` and `git diff --exit-code gen/` to catch generated code drift

---

## 5. Phase 2 — Backend Service

### 5.1 Authentication Middleware

Three auth tiers, all validated in the same middleware chain:

```go
// internal/auth/middleware.go

type Role int
const (
    RoleReader Role = iota  // any authenticated openteams.com user
    RoleWriter              // reader + valid push token header
    RoleAdmin               // reader + member of skillctl-admins Google Group
)

func NewMiddleware(cfg Config, sm *secretmanager.Client, ga *groupsAPI) connect.Interceptor {
    return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
        return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
            // 1. Validate Google ID token + domain
            claims, err := validateGoogleIDToken(ctx, extractBearer(req.Header()))
            if err != nil || claims.HostedDomain != cfg.AllowedDomain {
                return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid token"))
            }

            required := requiredRole(req.Spec().Procedure)

            // 2. Writer check
            if required >= RoleWriter {
                if err := validatePushToken(ctx, sm, req.Header().Get("X-Push-Token")); err != nil {
                    return nil, connect.NewError(connect.CodePermissionDenied, err)
                }
            }

            // 3. Admin check — Google Groups membership lookup
            if required >= RoleAdmin {
                ok, err := ga.IsMember(ctx, claims.Email, cfg.AdminGroupEmail)
                if err != nil || !ok {
                    return nil, connect.NewError(connect.CodePermissionDenied,
                        errors.New("admin group membership required"))
                }
            }

            return next(contextWithClaims(ctx, claims), req)
        }
    })
}

// Procedure → required role mapping
func requiredRole(procedure string) Role {
    switch {
    case strings.HasPrefix(procedure, "/skillctl.v1.FederationService/"):
        return RoleAdmin
    case strings.HasPrefix(procedure, "/skillctl.v1.RegistryService/Publish"),
         strings.HasPrefix(procedure, "/skillctl.v1.RegistryService/RecordInstall"):
        return RoleWriter
    default:
        return RoleReader
    }
}
```

**Admin group:** Create a Google Group `skillctl-admins@openteams.com` in Google Workspace Admin. Add admin users to the group. The backend checks membership via the Admin SDK Groups API at request time (cache the result for 5 minutes per email to avoid hammering the API).

### 5.2 In-Memory Cache with Firestore Listener

The cache now covers both internal skills and federated skills, with clear source labeling:

```go
// internal/registry/cache.go

type Cache struct {
    mu              sync.RWMutex
    internalSkills  map[string]*Skill          // keyed by name
    federatedSkills map[string]map[string]*Skill // keyed by marketplaceID → name
}

func NewCache(ctx context.Context, fs *firestore.Client) (*Cache, error) {
    c := &Cache{
        internalSkills:  make(map[string]*Skill),
        federatedSkills: make(map[string]map[string]*Skill),
    }

    // Load internal skills
    docs, _ := fs.Collection("skills").Documents(ctx).GetAll()
    // ... populate c.internalSkills

    // Load federated skills per marketplace
    mDocs, _ := fs.Collection("federated_skills").Documents(ctx).GetAll()
    // ... populate c.federatedSkills

    go c.listenForChanges(ctx, fs)
    go c.listenForFederatedChanges(ctx, fs)
    return c, nil
}
```

### 5.3 Full-Text Search (Federated-Aware)

```go
func (c *Cache) Search(q string, tags []string, sourceFilter SkillSource) []*Skill {
    c.mu.RLock()
    defer c.mu.RUnlock()

    var results []*Skill

    if sourceFilter != SkillSourceFederated {
        for _, s := range c.internalSkills {
            if matchesQuery(s, q) && matchesTags(s, tags) {
                results = append(results, s)
            }
        }
    }

    if sourceFilter != SkillSourceInternal {
        for _, mSkills := range c.federatedSkills {
            for _, s := range mSkills {
                if matchesQuery(s, q) && matchesTags(s, tags) {
                    results = append(results, s)
                }
            }
        }
    }

    // Internal skills sort above federated when score is equal
    sort.Slice(results, func(i, j int) bool {
        if results[i].Source != results[j].Source {
            return results[i].Source == SkillSourceInternal
        }
        return results[i].InstallCount > results[j].InstallCount
    })
    return results
}
```

### 5.4 Rate Limiting

```go
// internal/ratelimit/middleware.go
// Uses golang.org/x/time/rate — token bucket per key

type Limiter struct {
    readLimiters  sync.Map  // keyed by IP  — 100 req/min
    writeLimiters sync.Map  // keyed by push token — 10 req/min
    adminLimiters sync.Map  // keyed by email — 30 req/min (admin ops are heavier)
}
```

Also implement **client-side backoff** in the CLI: on `429`, wait `Retry-After` seconds (or 30s default) with jitter before retrying. Maximum 3 retries then surface error to user.

### 5.5 Dockerfile (Scratch Final Image)

```dockerfile
# Stage 1: build
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X main.version=$(cat VERSION)" \
    -o skillctl-server ./cmd/server

# Stage 2: security scan
FROM aquasec/trivy:latest AS scanner
COPY --from=builder /app/skillctl-server /skillctl-server
RUN trivy fs --exit-code 1 --severity HIGH,CRITICAL /skillctl-server

# Stage 3: final scratch image
FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/skillctl-server /skillctl-server
EXPOSE 8080
ENTRYPOINT ["/skillctl-server"]
```

Smoke test in CI:
```bash
docker run --rm -d -p 8080:8080 --name smoke $IMAGE
sleep 2
curl -f http://localhost:8080/healthz || (docker logs smoke && exit 1)
docker stop smoke
```

### 5.6 Blue/Green Deployment to Cloud Run

```yaml
- name: Deploy new revision (no traffic)
  run: |
    gcloud run deploy skillctl-api \
      --image ghcr.io/${{ github.repository }}/skillctl-backend:sha-${{ github.sha }} \
      --region us-central1 \
      --no-traffic \
      --tag canary

- name: Run smoke tests against canary
  run: |
    CANARY_URL=$(gcloud run services describe skillctl-api \
      --region us-central1 \
      --format 'value(status.traffic[?tag=="canary"].url)')
    go test ./e2e/backend/... -canary-url=$CANARY_URL -timeout 120s

- name: Cut over traffic (or rollback)
  run: |
    if [ "${{ steps.smoke.outcome }}" == "success" ]; then
      gcloud run services update-traffic skillctl-api --to-latest
    else
      REVISION=$(gcloud run revisions list --service skillctl-api \
        --filter="metadata.labels.canary=true" --format 'value(name)')
      gcloud run revisions delete $REVISION --quiet
      exit 1
    fi
```

---

## 6. Phase 3 — CLI

### 6.1 Cobra/Viper Root

```go
// cli/cmd/root.go
var rootCmd = &cobra.Command{
    Use:   "skillctl",
    Short: "Discover, install, and publish Claude Code skills",
    PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        return initConfig()
    },
}

func initConfig() error {
    viper.SetConfigName("config")
    viper.SetConfigType("yaml")
    viper.AddConfigPath("$HOME/.config/skillctl/")
    viper.AutomaticEnv()
    viper.SetEnvPrefix("SKILLCTL")
    return viper.ReadInConfig()
}
```

Config file: `~/.config/skillctl/config.yaml`

```yaml
api_url: https://skillctl-api-xxxx-uc.a.run.app
skills_dir: ~/.claude/skills
```

### 6.2 Google OAuth Device Flow

```go
// cli/internal/auth/device_flow.go

func Login(ctx context.Context) error {
    dc, err := requestDeviceCode(ctx)
    fmt.Printf("\nOpen this URL in your browser:\n  %s\n\nAnd enter code: %s\n\n",
        dc.VerificationURL, dc.UserCode)
    tok, err := pollForToken(ctx, dc)
    return saveToken(tok)
}
```

Token refresh is handled transparently before each request.

### 6.3 CLI Commands Specification

#### `skillctl explore`
```
skillctl explore [flags]

Flags:
  --tag string      Filter by tag (repeatable)
  --q string        Full-text search query
  --source string   Filter source: internal, external, all (default: all)
  --from string     Filter by specific marketplace ID
  --limit int       Max results (default 20)
  --json            Output as JSON

Output (default):
  SOURCE    NAME              OWNER           TAGS              INSTALLS  VERSION
  internal  data-pipeline     data-eng        data,spark        47        1.3.0
  external  code-review       anthropic       review,go         —         0.9.1
            [from: anthropic-official]
```

The `INSTALLS` column shows internal install counts only — we don't know how many times
an external skill has been installed from its upstream source, so we only track installs
that were made via skillctl.

#### `skillctl explore show <name>`
```
For internal skills: description, all versions, changelog, install count, owner, SKILL.md preview.
For federated skills: same + "Source: <marketplace display name>" + upstream URL.
```

#### `skillctl install <name>[@version] [--from <marketplace-id>]`
```
For internal skills:
  1. Fetches signed GCS download URL from backend
  2. Streams tar.gz to temp file
  3. Validates SHA256 checksum
  4. Unpacks to ~/.claude/skills/<name>/
  5. Calls RecordInstall RPC (best-effort)

For federated skills:
  1. Backend proxies the download from the upstream source (GitHub release, raw archive, etc.)
     and returns a short-lived signed URL pointing to a temp-cached copy in GCS
  2. Same download + checksum + unpack flow as internal
  3. RecordInstall records marketplace_id + upstream skill name

Name collision (internal + federated have same name):
  - Prefer internal by default
  - Dev must use --from <marketplace-id> to explicitly install the external one
  - CLI prints a warning: "An internal skill named X exists. Use --from to install
    the external version, or omit --from to install the internal one."
```

#### `skillctl push <path> [--draft] [--changelog "..."]`
```
1. Reads and validates SKILL.md frontmatter (name, description, version required)
2. Checks version is valid semver and doesn't already exist in registry
3. Packages directory as tar.gz
4. POSTs to PublishSkill RPC with metadata
5. Backend uploads to GCS and registers in Firestore
6. If --draft: uploaded but not listed in explore results
```

#### `skillctl auth login / logout / status`
```
login   — runs device flow, saves token
logout  — deletes ~/.config/skillctl/credentials.json
status  — prints current user email, token expiry, admin group membership, backend connection
```

#### `skillctl marketplace` (admin only — returns PermissionDenied if not in admin group)
```
skillctl marketplace list
  Lists all whitelisted marketplaces with status, skill count, last sync time.

skillctl marketplace add <url> [flags]
  --name string       Display name (required)
  --type string       github-repo | agentskills-io (default: github-repo)
  --mode string       all | specific (default: all)
  --sync int          Sync interval in minutes (default: 60)
  Adds a marketplace to the whitelist. Triggers an immediate sync.
  Prints the assigned marketplace ID on success.

skillctl marketplace remove <marketplace-id>
  Removes from whitelist. Federated skills from this marketplace are immediately
  removed from the cache and can no longer be installed.
  Prompts for confirmation unless --force is passed.

skillctl marketplace sync <marketplace-id>
  Triggers an immediate out-of-cycle sync for one marketplace.
  Useful after adding a new skill to a whitelisted upstream repo.

skillctl marketplace allow <marketplace-id> <skill-name>
  When a marketplace is in SPECIFIC mode, adds a skill to its allowlist.

skillctl marketplace block <marketplace-id> <skill-name>
  Removes a skill from the allowlist (or adds it to a blocklist in ALL mode).

skillctl marketplace show <marketplace-id>
  Shows marketplace details + full list of synced skills + their allowlist status.
```

#### `skillctl update`
```
1. Calls GetLatestCLIVersion RPC
2. Compares to embedded version string
3. If newer: downloads platform-specific binary from GitHub Releases
4. Verifies SHA256 checksum
5. Atomically replaces current binary
6. Exec's new binary with same args to confirm it starts
```

### 6.4 `.goreleaser.yml` (CLI)

```yaml
version: 2

before:
  hooks:
    - go mod tidy
    - go generate ./...

builds:
  - id: skillctl
    dir: cli/
    binary: skillctl
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.Commit}}
      - -X main.date={{.Date}}
    env:
      - CGO_ENABLED=0
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    ignore:
      - goos: windows
        goarch: arm64

archives:
  - name_template: "skillctl_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    format_overrides:
      - goos: windows
        format: zip

checksum:
  name_template: "skillctl_{{ .Version }}_checksums.txt"

scoop:
  name: skillctl
  bucket:
    owner: yourorg
    name: scoop-bucket
  homepage: https://github.com/yourorg/skillctl
  description: "Discover, install, and publish Claude Code skills"
  license: MIT

release:
  draft: false
  prerelease: auto

changelog:
  sort: asc
  filters:
    exclude: ["^docs:", "^test:", "^chore:"]
```

---

## 7. Phase 4 — Infrastructure (OpenTofu)

### 7.1 State Backend

```hcl
terraform {
  backend "gcs" {
    bucket = "skillctl-tfstate"
    prefix = "tofu/state"
  }
  required_providers {
    google = { source = "hashicorp/google", version = "~> 5.0" }
  }
}
```

### 7.2 Cloud Run Module

```hcl
resource "google_cloud_run_v2_service" "api" {
  name     = "skillctl-api-${var.env_name}"
  location = var.region

  template {
    scaling {
      min_instance_count = var.min_instances
      max_instance_count = var.max_instances
    }
    containers {
      image = var.image
      env { name = "ALLOWED_DOMAIN";     value = var.allowed_domain }
      env { name = "GCS_BUCKET";         value = "skillctl-skills-${var.env_name}" }
      env { name = "ENV";                value = var.env_name }
      env { name = "ADMIN_GROUP_EMAIL";  value = var.admin_group_email }
      env {
        name = "PUSH_TOKENS"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.push_tokens.secret_id
            version = "latest"
          }
        }
      }
      liveness_probe { http_get { path = "/healthz" }; period_seconds = 10 }
      startup_probe  { http_get { path = "/healthz" }; failure_threshold = 3; period_seconds = 5 }
    }
  }
  traffic { type = "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST"; percent = 100 }
}
```

New variable: `admin_group_email = "skillctl-admins@openteams.com"` — passed to the backend so it knows which group to check.

The Cloud Run service account needs `roles/admin.directory.groups.readonly` on the Google Workspace domain so it can call the Groups API for admin checks.

### 7.3 Firestore Indexes

The `infra/modules/firestore/` module must create composite indexes for the new collections:

```hcl
# marketplaces collection — no composite indexes needed (small collection, full scan is fine)

# federated_skills collection
resource "google_firestore_index" "federated_by_marketplace" {
  collection = "federated_skills"
  fields {
    field_path = "marketplace_id"
    order      = "ASCENDING"
  }
  fields {
    field_path = "name"
    order      = "ASCENDING"
  }
}
```

### 7.4 Environments

`infra/envs/dev/terraform.tfvars`:
```hcl
env_name           = "dev"
min_instances      = 0
max_instances      = 2
admin_group_email  = "skillctl-admins@openteams.com"
image              = "ghcr.io/yourorg/skillctl-backend:latest"
```

`infra/envs/prod/terraform.tfvars`:
```hcl
env_name           = "prod"
min_instances      = 1
max_instances      = 5
admin_group_email  = "skillctl-admins@openteams.com"
image              = "ghcr.io/yourorg/skillctl-backend:latest"
```

### 7.5 tfsec

Run `tfsec` against `infra/`. Acceptable suppressions (must have inline `#tfsec:ignore` + reason):
- `AVD-GCP-0069` (Cloud Run public access) — auth is app-layer

---

## 8. Phase 5 — CI/CD Pipelines

### 8.1 CLI Pipeline (`ci-cli.yml`)

```
Stage 1: Quality (parallel)
├── gitleaks
├── golangci-lint
└── buf-check (lint + breaking)

Stage 2: Unit Tests
└── go test ./cli/... -race -coverprofile=coverage.out

Stage 3: Build (parallel per platform)
├── linux/amd64, linux/arm64
├── darwin/amd64, darwin/arm64
└── windows/amd64
Each: CGO_ENABLED=0 go build → upload as GitHub Actions artifact

Stage 4: E2E Tests (parallel per platform)
├── e2e-linux-amd64
├── e2e-darwin-amd64, e2e-darwin-arm64
└── e2e-windows-amd64
NOTE: Includes marketplace_test.go — requires a test admin token in CI secrets
      Uses a seeded test marketplace (a public GitHub repo with a fixture skill)
      that is pre-added to the dev environment whitelist

Stage 5: Release (on tag vX.Y.Z)
└── goreleaser release → GitHub Release + Scoop manifest PR

Stage 6: Docs (on merge to main)
└── skillctl docs → gh-pages branch
```

### 8.2 Backend Pipeline (`ci-backend.yml`)

```
Stage 1: Quality (parallel)
├── gitleaks
├── golangci-lint
└── buf-check

Stage 2: Unit Tests
└── go test ./backend/... -race

Stage 3: Container Build
└── docker build → Trivy scan → smoke test → push sha-tagged image

Stage 4: E2E + DAST
├── Deploy canary to dev Cloud Run
├── Run e2e/backend/ smoke suite (includes federation endpoint tests)
├── OWASP ZAP baseline scan (targets /skillctl.v1.FederationService/ endpoints too)
└── Cut traffic if pass / rollback if fail

Stage 5: Release (on merge to main)
└── goreleaser → ghcr.io tag :latest + :vX.Y.Z
```

### 8.3 Infrastructure Pipeline (`ci-infra.yml`)

```
Stage 1: Scan
└── tfsec infra/ --minimum-severity MEDIUM

Stage 2: Plan (on PR — both envs in parallel)
├── tofu -chdir=infra/envs/dev  init + plan -out=dev.tfplan
├── tofu -chdir=infra/envs/prod init + plan -out=prod.tfplan
└── Post formatted plan as PR comment (peter-evans/create-or-update-comment)

Stage 3: Apply (on merge to main)
├── tofu -chdir=infra/envs/dev  apply dev.tfplan
└── tofu -chdir=infra/envs/prod apply prod.tfplan
```

### 8.4 Proto Pipeline (`ci-proto.yml`)

```
Stage 1:
├── buf lint
├── buf breaking (warn or block based on VERSION)
└── buf generate + git diff --exit-code gen/
```

### 8.5 Golangci-lint (`.golangci.yml`)

```yaml
linters:
  enable:
    - errcheck, staticcheck, govet, gofmt, goimports
    - revive, gosec, prealloc, misspell, unconvert
  disable:
    - exhaustruct

linters-settings:
  gosec:
    excludes: [G104]

issues:
  exclude-rules:
    - path: "gen/"
      linters: [all]
    - path: "_test.go"
      linters: [gosec]
```

### 8.6 GitHub Actions Required Secrets

| Secret | Description |
|---|---|
| `GCP_SERVICE_ACCOUNT_KEY_DEV` | Cloud Run deploy + Firestore + GCS + Admin SDK access (dev) |
| `GCP_SERVICE_ACCOUNT_KEY_PROD` | Same for prod |
| `GCS_TFSTATE_SA_KEY` | OpenTofu state bucket |
| `GHCR_TOKEN` | Push to GitHub Container Registry |
| `GOOGLE_OAUTH_CLIENT_ID` | Google OAuth client ID |
| `SCOOP_BUCKET_TOKEN` | PAT for scoop-bucket repo |
| `SKILLCTL_TEST_ADMIN_TOKEN` | ID token for a test admin account used in e2e marketplace tests |
| `SKILLCTL_TEST_PUSH_TOKEN` | Push token for e2e publish tests |

---

## 9. Phase 6 — Federation & Marketplace Management

This phase builds the federation system on top of the core registry.

### 9.1 Firestore Schema — New Collections

#### `marketplaces/{id}`

```
id:                    "anthropic-official"
display_name:          "Anthropic Official Skills"
source_url:            "https://github.com/anthropics/skills"
type:                  "GITHUB_REPO"
mode:                  "ALL"               // or "SPECIFIC"
allowed_skills:        []                  // populated when mode = SPECIFIC
sync_interval_minutes: 60
last_synced_at:        <timestamp>
skill_count:           42
enabled:               true
added_by:              "alice@openteams.com"
added_at:              <timestamp>
```

#### `federated_skills/{marketplace_id}/{skill_name}`

```
name:           "pdf"
description:    "..."
tags:           ["pdf", "documents"]
marketplace_id: "anthropic-official"
upstream_url:   "https://github.com/anthropics/skills/tree/main/skills/pdf"
latest_version: "0.9.1"
allowed:        true   // false if blocked by admin after marketplace-level allow
synced_at:      <timestamp>
```

### 9.2 Federation Poller

The poller runs as a goroutine inside the Cloud Run instance. On Cloud Run, only one instance
should drive writes to avoid redundant sync work — use a Firestore distributed lock (a document
with a TTL field that the instance must claim before syncing, and refreshes while running).

```go
// internal/federation/poller.go

type Poller struct {
    fs     *firestore.Client
    cache  *registry.Cache
    fetchers map[MarketplaceType]Fetcher
}

type Fetcher interface {
    // FetchSkills returns all skills currently in the external marketplace.
    // The poller diffs against what's in Firestore and applies changes.
    FetchSkills(ctx context.Context, m *Marketplace) ([]*FederatedSkill, error)
}

func (p *Poller) Run(ctx context.Context) {
    for {
        marketplaces, _ := p.loadEnabledMarketplaces(ctx)
        for _, m := range marketplaces {
            if time.Since(m.LastSyncedAt) >= time.Duration(m.SyncIntervalMinutes)*time.Minute {
                p.syncMarketplace(ctx, m)
            }
        }
        time.Sleep(1 * time.Minute) // check interval
    }
}

func (p *Poller) syncMarketplace(ctx context.Context, m *Marketplace) {
    fetcher := p.fetchers[m.Type]
    skills, err := fetcher.FetchSkills(ctx, m)
    if err != nil {
        log.Printf("sync failed for %s: %v", m.ID, err)
        return
    }

    // Apply skill-level allowlist filtering
    skills = p.applyAllowlist(m, skills)

    // Diff + write to Firestore
    p.applyDiff(ctx, m, skills)

    // Update last_synced_at + skill_count
    p.fs.Collection("marketplaces").Doc(m.ID).Update(ctx, []firestore.Update{
        {Path: "last_synced_at", Value: time.Now()},
        {Path: "skill_count",    Value: len(skills)},
    })
}
```

### 9.3 GitHub Fetcher

```go
// internal/federation/github.go

type GitHubFetcher struct {
    httpClient *http.Client
    // Uses unauthenticated GitHub API for public repos.
    // If the repo is private (future use case), inject a GitHub token via Secret Manager.
}

// For a GitHub repo marketplace, fetches the tree at the configured ref,
// finds all SKILL.md files, reads their frontmatter, and returns FederatedSkill structs.
// Does not download archives — only reads metadata. Archives are fetched on-demand
// when a dev runs `skillctl install`.
func (f *GitHubFetcher) FetchSkills(ctx context.Context, m *Marketplace) ([]*FederatedSkill, error) {
    // GET https://api.github.com/repos/{owner}/{repo}/git/trees/HEAD?recursive=1
    // Filter entries ending in /SKILL.md
    // For each: GET raw content, parse YAML frontmatter
    // Return slice of FederatedSkill
}
```

GitHub API rate limits: 60 unauthenticated requests/hour per IP. For a 60-minute sync interval with <60 marketplaces this is fine. If you add many marketplaces or reduce the interval, inject a GitHub token (stored in Secret Manager) to get 5,000 req/hour.

### 9.4 Install Proxy for Federated Skills

When a dev installs an external skill, the backend:

1. Verifies the skill is in the `federated_skills` collection (i.e., it was approved by the whitelist)
2. Fetches the archive from the upstream source (GitHub archive URL, etc.)
3. Validates the archive contains a valid `SKILL.md`
4. Stores a temp copy in GCS at `federated-cache/{marketplace_id}/{name}/{version}.tar.gz` (TTL: 24h)
5. Returns a signed GCS URL to the CLI

This two-step approach means:
- The whitelist is enforced at download time, not just discovery time
- If a skill is removed from the whitelist between `explore` and `install`, the install is blocked
- GCS caching avoids re-fetching the same archive repeatedly

### 9.5 Skill Name Collision Policy

If an internal skill and a federated skill share the same name:

- `skillctl explore` lists both, clearly labeled by source
- `skillctl install <name>` prefers the internal skill and prints a warning
- `skillctl install <name> --from <marketplace-id>` installs the federated version explicitly
- The backend never silently shadow an internal skill with a federated one

### 9.6 Pre-Seeded Marketplace Whitelist

On first deployment, seed the whitelist via a one-time migration script (not in OpenTofu — this is data, not infra):

```bash
# scripts/seed-marketplaces.sh
skillctl marketplace add https://github.com/anthropics/skills \
  --name "Anthropic Official Skills" \
  --mode all \
  --sync 60

skillctl marketplace add https://github.com/obra/superpowers \
  --name "Superpowers" \
  --mode specific \
  --sync 120

# Then explicitly allow the superpowers skills your org has reviewed:
skillctl marketplace allow superpowers brainstorm
skillctl marketplace allow superpowers write-plan
skillctl marketplace allow superpowers execute-plan
```

Document this script in the repo. New approved marketplaces are added by running `skillctl marketplace add` — no deployment required.

### 9.7 Audit Log

Every admin action (add/remove marketplace, allow/block skill) is written to a Firestore `audit_log` collection:

```
audit_log/{auto-id}
  action:       "MARKETPLACE_ADDED"
  actor:        "alice@openteams.com"
  target_id:    "anthropic-official"
  details:      { source_url: "...", mode: "ALL" }
  timestamp:    <timestamp>
```

This is append-only. No deletion. Admins can query it via `skillctl marketplace audit` (future feature — leave as a stub for now).

---

## 10. Phase 7 — Dogfood Skill

Create the first internal skill in the registry to validate the full pipeline end-to-end and give devs something immediately useful.

**`skills/goreleaser/SKILL.md`**

```yaml
---
name: goreleaser
description: >
  Expert guidance on GoReleaser configuration for cross-platform Go binary releases,
  container publishing, Scoop/Homebrew manifests, and GitHub Actions integration.
  Use this skill whenever the user asks about releasing Go binaries, cross-compilation,
  GoReleaser config, .goreleaser.yml, or publishing Go tools to package managers.
version: 0.1.0
owner: platform
tags: [go, release, ci, goreleaser, packaging]
---

# GoReleaser Skill

[Full skill body — GoReleaser best practices, common config patterns,
 multi-platform build matrix, container publishing, Scoop manifest setup, etc.]
```

Push this as part of the initial setup so new team members can `skillctl install goreleaser` immediately.

---

## 11. Secrets & Configuration Reference

### GCP IAM — Service Accounts

| SA | Roles | Used by |
|---|---|---|
| `skillctl-backend@...` | `roles/datastore.user`, `roles/storage.objectAdmin`, `roles/secretmanager.secretAccessor`, `roles/admin.directory.groups.readonly` | Cloud Run runtime |
| `skillctl-deployer@...` | `roles/run.developer`, `roles/storage.objectAdmin` | CI deploy job |
| `skillctl-tofu@...` | `roles/editor` (narrow later) | OpenTofu apply |

Note: `roles/admin.directory.groups.readonly` is a Google Workspace role, not a GCP role. It must be granted in Google Workspace Admin console, not via OpenTofu.

### Environment Variables — Backend

| Var | Description |
|---|---|
| `ALLOWED_DOMAIN` | `openteams.com` |
| `GCS_BUCKET` | `skillctl-skills-{env}` |
| `FIRESTORE_PROJECT` | GCP project ID |
| `PORT` | `8080` |
| `ENV` | `dev` or `prod` |
| `PUSH_TOKENS` | JSON object mapping token→team, from Secret Manager |
| `ADMIN_GROUP_EMAIL` | `skillctl-admins@openteams.com` |
| `GROUPS_API_CACHE_TTL_SEC` | `300` (5 min cache for group membership checks) |
| `FEDERATION_POLL_INTERVAL_SEC` | `60` (how often the poller wakes to check sync schedules) |
| `GITHUB_TOKEN` | Optional; Secret Manager. Raises GitHub API rate limit for federation sync. |

---

## 12. Outstanding Decisions & Constraints

1. **Google OAuth Client ID** — Create an OAuth 2.0 client in GCP Console under openteams.com Workspace. Set authorized redirect URI to `urn:ietf:wg:oauth:2.0:oob` for device flow. Client ID is safe to embed in the binary.

2. **Admin Google Group** — Create `skillctl-admins@openteams.com` in Google Workspace Admin before deploying. The backend SA needs `roles/admin.directory.groups.readonly` granted in Workspace Admin, not GCP IAM.

3. **Scoop bucket** — Requires a separate public GitHub repo `yourorg/scoop-bucket`. Create before first GoReleaser run.

4. **Proto versioning** — The buf breaking-change block on >= 1.0.0 reads a `VERSION` file in the repo root. Maintain it; it should match the latest git tag.

5. **GCS bucket naming** — GCS names are globally unique. Use `ot-skillctl-skills-dev` / `ot-skillctl-skills-prod`. The federated cache bucket can share the same bucket under a `federated-cache/` prefix.

6. **OWASP ZAP + ConnectRPC** — ZAP targets the JSON/HTTP mode. Ensure `/healthz` is unauthenticated and configure `zap-baseline.yaml` with the ConnectRPC JSON endpoint paths, including the new FederationService endpoints.

7. **Windows Defender** — Scoop distribution bypasses SmartScreen. Document that direct `.exe` downloads require right-click → "Run anyway".

8. **Self-update on Windows** — Use `inconshreveable/go-update` which handles the locked-binary swap via a temp file + batch wrapper.

9. **Firestore cold-start latency** — The federation poller and cache load add ~1s to cold start. Accept for dev (min_instances=0). Prod uses min_instances=1.

10. **Generated code in git** — Add `.gitattributes` entry `gen/** linguist-generated=true` to collapse generated code in GitHub PR diffs.

11. **GitHub API rate limits for federation** — Unauthenticated: 60 req/hour per IP. Cloud Run instances share the same egress IP pool. If you add >10 marketplaces with short sync intervals, inject a GitHub token via Secret Manager. Add `GITHUB_TOKEN` to the env var list above and document that it's optional but recommended at scale.

12. **Private external marketplaces** — The current design assumes all whitelisted marketplaces are public GitHub repos. If a future marketplace is a private repo (e.g., a partner org's skill repo), the GitHub fetcher needs to support injecting a per-marketplace PAT. Design the fetcher interface to accept optional auth credentials from the marketplace document — leave a `auth_secret_name` field in the Firestore schema as a placeholder even if it's unused initially.

13. **Federated skill version pinning** — The poller tracks `latest_version` for each external skill. `skillctl install <name>@<version>` for a federated skill requires the poller to have synced that specific version. Add a `versions` subcollection under `federated_skills/{marketplace_id}/{name}` mirroring the internal skill version structure.

14. **Rate limit values** — Starting values: 100 req/min per IP (reads), 10 req/min per push token (writes), 30 req/min per email (admin). Store as env vars so they can be changed without a redeploy.

---

*End of work plan. Implement phases in order: 1 → 2 → 3 → 4 → 5 → 6 → 7. Phase 6 (Federation) depends on Phase 2 (backend) and Phase 3 (CLI) being complete. The federation admin commands in the CLI are thin wrappers over the FederationService RPCs — the bulk of the work is server-side in the poller and fetchers.*
