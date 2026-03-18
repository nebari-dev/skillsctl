# skillsctl - Iterative Build Plan (v2 Architecture)

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build skillsctl (CLI + backend registry for Claude Code skills) in thin vertical slices, each delivering testable end-to-end functionality before moving on. Based on v2 workplan (K8s-native, PostgreSQL, Valkey, OCI, generic OIDC, Helm chart).

**Architecture:** ConnectRPC backend (Go) deployed via Helm chart on Kubernetes. PostgreSQL (CloudNativePG) for persistence, Valkey for cache invalidation pub/sub, OCI registry (ghcr.io) for skill archives via oras-go. Cobra/Viper CLI communicates via ConnectRPC. Generic OIDC device flow for auth (works with Keycloak, Okta, Dex, etc.). Optional Nebari integration via NebariApp CRD.

**Tech Stack:** Go 1.25+, ConnectRPC, Protocol Buffers (buf), Cobra/Viper, PostgreSQL (pgx), Valkey, oras-go, goose (migrations), Helm, GoReleaser, GitHub Actions

**Reference:** Full architecture in `skillsctl-workplan-v2.md`

---

## Slice Overview

Each slice is a vertical cut delivering testable value:

| Slice | What it delivers | How you test it |
|-------|-----------------|-----------------|
| 1 | Project scaffolding + tooling | `go build ./...`, `buf lint`, `golangci-lint run` |
| 2 | Proto definitions + code generation | Generated Go types compile, `buf lint` passes |
| 3 | Backend healthz + ListSkills (in-memory) | `curl localhost:8080/healthz`, `curl` ListSkills JSON endpoint |
| 4 | CLI explore + show | `skillsctl explore` prints table against running backend |
| 5 | PostgreSQL store + goose migrations | Backend reads/writes skills to PostgreSQL, survives restart |
| 6 | Auth - OIDC middleware + device flow | `skillsctl auth login` works, unauthenticated requests rejected |
| 7 | Publish flow (push + OCI storage) | `skillsctl push ./my-skill/` uploads, appears in explore |
| 8 | Install flow (OCI pull) | `skillsctl install <name>` pulls OCI artifact + unpacks |
| 9 | Search | `skillsctl explore --q "data"` filters results |
| 10 | Valkey cache invalidation | Publish on instance A, instance B cache updates |
| 11 | Helm chart | `helm install skillsctl ./chart` works on kind cluster |
| 12 | CI/CD pipelines | PR triggers lint+test, merge triggers build+push |
| 13 | Rate limiting | Burst requests get 429, CLI backs off |
| 14 | Self-update | `skillsctl update` downloads new binary |
| 15 | Federation | `skillsctl marketplace add <url>`, federated skills in explore |
| 16 | Dogfood skill | `skillsctl install goreleaser` works end-to-end |

---

## Chunk 1: Project Scaffolding

### Task 1.1: Go Module + Directory Structure

**Files:**
- Create: `go.mod`
- Create: `.gitignore`
- Create: `.gitattributes`
- Create: `Makefile`
- Create: `VERSION`
- Create: `README.md`

- [ ] **Step 1: Initialize Go module** (already done)

```bash
go mod init github.com/nebari-dev/skillsctl
```

- [ ] **Step 2: Create .gitignore**

```gitignore
# Binaries
skillsctl
skillsctl-server
*.exe

# IDE
.idea/
.vscode/
*.swp

# OS
.DS_Store
Thumbs.db

# Test
coverage.out
*.test

# Build
dist/

# Config (local)
CLAUDE.md
```

- [ ] **Step 3: Create .gitattributes**

```
gen/** linguist-generated=true
```

- [ ] **Step 4: Create VERSION**

```
0.1.0
```

- [ ] **Step 5: Create Makefile**

```makefile
.PHONY: proto lint test build-cli build-backend clean

proto:
	buf lint proto/
	buf generate proto/

lint:
	golangci-lint run ./...

test:
	go test ./... -race -coverprofile=coverage.out

test-backend:
	go test ./backend/... -race -coverprofile=coverage.out

test-cli:
	go test ./cli/... -race -coverprofile=coverage.out

build-cli:
	CGO_ENABLED=0 go build -o skillsctl ./cli

build-backend:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o skillsctl-server ./backend/cmd/server

clean:
	rm -f skillsctl skillsctl-server coverage.out
```

- [ ] **Step 6: Create README.md**

Basic project README explaining what skillsctl is, how to build, test, and contribute. Reference the workplan for architecture details.

- [ ] **Step 7: Verify go module**

Run: `go mod tidy`
Expected: clean exit

- [ ] **Step 8: Commit**

```bash
git add go.mod .gitignore .gitattributes Makefile VERSION README.md
git commit -m "feat: initialize go module and project scaffolding"
```

### Task 1.2: Buf + Proto Setup

**Files:**
- Create: `proto/buf.yaml`
- Create: `proto/buf.gen.yaml`
- Create: `proto/skillsctl/v1/skill.proto`
- Create: `proto/skillsctl/v1/registry.proto`

- [ ] **Step 1: Create buf.yaml**

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

- [ ] **Step 2: Create buf.gen.yaml**

```yaml
version: v2
managed:
  enabled: true
  override:
    - file_option: go_package_prefix
      value: github.com/nebari-dev/skillsctl/gen/go
plugins:
  - remote: buf.build/protocolbuffers/go
    out: gen/go
    opt:
      - paths=source_relative

  - remote: buf.build/connectrpc/go
    out: gen/go
    opt:
      - paths=source_relative
```

- [ ] **Step 3: Create skill.proto with core types**

File: `proto/skillsctl/v1/skill.proto`

```protobuf
syntax = "proto3";
package skillsctl.v1;

import "google/protobuf/timestamp.proto";

option go_package = "github.com/nebari-dev/skillsctl/gen/go/skillsctl/v1;skillsctlv1";

enum SkillSource {
  SKILL_SOURCE_UNSPECIFIED = 0;
  SKILL_SOURCE_INTERNAL = 1;
  SKILL_SOURCE_FEDERATED = 2;
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
  SkillSource source = 9;
  string marketplace_id = 10;
  string upstream_url = 11;
}

message SkillVersion {
  string version = 1;
  string changelog = 2;
  string oci_ref = 3;
  string digest = 4;
  int64 size_bytes = 5;
  string published_by = 6;
  google.protobuf.Timestamp published_at = 7;
  bool draft = 8;
}
```

- [ ] **Step 4: Create registry.proto with RegistryService**

File: `proto/skillsctl/v1/registry.proto`

Start with only read RPCs. Write RPCs added in Slice 7.

```protobuf
syntax = "proto3";
package skillsctl.v1;

import "skillsctl/v1/skill.proto";

option go_package = "github.com/nebari-dev/skillsctl/gen/go/skillsctl/v1;skillsctlv1";

service RegistryService {
  rpc ListSkills(ListSkillsRequest) returns (ListSkillsResponse);
  rpc GetSkill(GetSkillRequest) returns (GetSkillResponse);
}

message ListSkillsRequest {
  repeated string tags = 1;
  SkillSource source_filter = 2;
  int32 page_size = 3;
  string page_token = 4;
}

message ListSkillsResponse {
  repeated Skill skills = 1;
  string next_page_token = 2;
}

message GetSkillRequest {
  string name = 1;
}

message GetSkillResponse {
  Skill skill = 1;
  repeated SkillVersion versions = 2;
}
```

- [ ] **Step 5: Run buf lint**

Run: `buf lint proto/`
Expected: clean exit, no warnings

- [ ] **Step 6: Generate Go code**

Run: `buf generate proto/`
Expected: files created under `gen/go/skillsctl/v1/`

- [ ] **Step 7: Verify generated code compiles**

Run: `go mod tidy && go build ./gen/...`
Expected: clean exit

- [ ] **Step 8: Commit**

```bash
git add proto/ gen/ go.mod go.sum
git commit -m "feat: protobuf definitions and generated ConnectRPC code"
```

### Task 1.3: Linter Configuration

**Files:**
- Create: `.golangci.yml`
- Create: `.gitleaks.toml`

- [ ] **Step 1: Create .golangci.yml (v2 format)**

```yaml
version: "2"
linters:
  enable:
    - errcheck
    - staticcheck
    - govet
    - goimports
    - revive
    - gosec
    - misspell
    - unconvert
  settings:
    gosec:
      excludes:
        - G104
exclusions:
  paths:
    - gen/
  rules:
    - path: "_test\\.go"
      linters:
        - gosec
```

- [ ] **Step 2: Create .gitleaks.toml**

```toml
[allowlist]
  paths = [
    '''gen/''',
    '''\.git/''',
  ]
```

- [ ] **Step 3: Verify linters pass**

Run: `golangci-lint run ./...`
Expected: clean exit

Run: `gitleaks detect`
Expected: no leaks detected

- [ ] **Step 4: Commit**

```bash
git add .golangci.yml .gitleaks.toml
git commit -m "feat: golangci-lint and gitleaks configuration"
```

---

## Chunk 2: Backend - Health Check + ListSkills (In-Memory)

### Task 2.1: Backend Server Skeleton + /healthz

**Files:**
- Create: `backend/cmd/server/main.go`
- Create: `backend/internal/server/server.go`
- Create: `backend/internal/server/server_test.go`

TDD: write failing test for healthz, implement, verify.

### Task 2.2: Store Interface + In-Memory Implementation

**Files:**
- Create: `backend/internal/store/store.go` (SkillStore interface)
- Create: `backend/internal/store/memory.go`
- Create: `backend/internal/store/memory_test.go`

SkillStore interface with ListSkills and GetSkill. Memory implementation for local dev and testing.

### Task 2.3: ConnectRPC Registry Service

**Files:**
- Create: `backend/internal/registry/service.go`
- Create: `backend/internal/registry/service_test.go`
- Modify: `backend/internal/server/server.go` (wire registry handler)
- Modify: `backend/internal/server/server_test.go`
- Modify: `backend/cmd/server/main.go` (seed test data)

TDD: test ListSkills and GetSkill via ConnectRPC client against test server. Wire into server. Smoke test with curl.

---

## Chunk 3: CLI - explore + show

### Task 3.1: Cobra Root + Viper Config

**Files:**
- Create: `cli/main.go`
- Create: `cli/cmd/root.go`
- Create: `cli/internal/config/config.go`
- Create: `cli/internal/config/config_test.go`

Config defaults: api_url=http://localhost:8080, skills_dir=~/.claude/skills, auth.oidc_issuer and auth.client_id empty.

### Task 3.2: ConnectRPC Client Wrapper

**Files:**
- Create: `cli/internal/api/client.go`
- Create: `cli/internal/api/client_test.go`

Typed wrapper around the generated ConnectRPC client. Tests use a real test server (backend registry + memory store).

### Task 3.3: `skillsctl explore` + `skillsctl explore show` Commands

**Files:**
- Create: `cli/cmd/explore.go`
- Create: `cli/cmd/explore_test.go`

Table output for explore, detail view for show. Tests verify output against test backend.

---

## Chunk 4: PostgreSQL Store + Migrations

### Task 4.1: Goose Migrations

**Files:**
- Create: `backend/internal/store/migrations/001_initial.sql`

Creates skills and skill_versions tables per v2 workplan.

### Task 4.2: PostgreSQL Store Implementation

**Files:**
- Create: `backend/internal/store/postgres.go`
- Create: `backend/internal/store/postgres_test.go`

Implements SkillStore interface using pgx. Tests run against a real PostgreSQL (dockertest or testcontainers).

### Task 4.3: Wire PostgreSQL into Server

**Files:**
- Modify: `backend/cmd/server/main.go`

Reads DATABASE_URL env var. Falls back to memory store if not set (for local dev without DB).

---

## Chunk 5: Auth - OIDC Middleware + Device Flow

### Task 5.1: OIDC Token Validator

**Files:**
- Create: `backend/internal/auth/oidc.go`
- Create: `backend/internal/auth/oidc_test.go`

Generic OIDC: fetch JWKS from issuer, validate JWT signature/expiry/audience, extract claims including groups.

### Task 5.2: Auth Middleware

**Files:**
- Create: `backend/internal/auth/middleware.go`
- Create: `backend/internal/auth/middleware_test.go`

ConnectRPC interceptor. Three roles: Reader (any valid token), Writer (token + push token header), Admin (token with admin group in groups claim).

### Task 5.3: CLI Auth Commands + Device Flow

**Files:**
- Create: `cli/internal/auth/device_flow.go`
- Create: `cli/internal/auth/token_store.go`
- Create: `cli/internal/auth/token_store_test.go`
- Create: `cli/cmd/auth.go`

Generic OIDC device flow (RFC 8628). Discovers endpoints from issuer's .well-known/openid-configuration. Token cached at ~/.config/skillsctl/credentials.json.

---

## Chunk 6: Publish Flow (Push + OCI Storage)

### Task 6.1: Add PublishSkill + GetDownloadURL RPCs to Proto

Modify registry.proto, regenerate.

### Task 6.2: SKILL.md Validation + Tar.gz Packaging

**Files:**
- Create: `cli/internal/skill/validate.go`, `validate_test.go`
- Create: `cli/internal/skill/package.go`, `package_test.go`

### Task 6.3: OCI Store (oras-go)

**Files:**
- Create: `backend/internal/store/oci.go`, `oci_test.go`

Push skill archives as OCI artifacts. Generate short-lived pull tokens.

### Task 6.4: PublishSkill Handler + `skillsctl push`

Wire it all together. End-to-end: push a skill, see it in explore.

---

## Chunk 7: Install Flow (OCI Pull)

### Task 7.1: GetDownloadURL Handler

Returns OCI reference + pull token + digest.

### Task 7.2: OCI Pull in CLI

**Files:**
- Create: `cli/internal/oci/pull.go`, `pull_test.go`
- Create: `cli/cmd/install.go`, `install_test.go`

Pull OCI artifact via oras-go, verify digest, extract to ~/.claude/skills/<name>/.

### Task 7.3: RecordInstall RPC

Increment install_count. CLI calls best-effort after successful install.

---

## Chunk 8: Search

Add SearchSkills RPC to proto. Implement full-text search (name, description, tags). Wire --q flag in explore.

---

## Chunk 9: Valkey Cache Invalidation

### Task 9.1: Valkey Pub/Sub

**Files:**
- Create: `backend/internal/cache/valkey.go`, `valkey_test.go`

Publish invalidation on writes, subscribe on all instances, drop affected cache entries.

### Task 9.2: In-Memory Cache with Valkey Integration

**Files:**
- Create: `backend/internal/registry/cache.go`, `cache_test.go`

Load from PostgreSQL on startup. Valkey subscriber invalidates entries. Reload from DB on cache miss.

---

## Chunk 10: Helm Chart

### Task 10.1: Chart Structure

**Files:**
- Create: `chart/Chart.yaml`, `chart/values.yaml`, `chart/values.schema.json`
- Create: `chart/templates/` (deployment, service, configmap, secret, serviceaccount, hpa, pdb, _helpers.tpl, NOTES.txt)
- Create: `chart/templates/nebariapp.yaml` (conditional on nebari.enabled)
- Create: `chart/ci/test-values.yaml`

### Task 10.2: Test on kind Cluster

`helm lint`, `helm template`, `helm install` on kind with CloudNativePG operator pre-installed.

---

## Chunk 11: CI/CD Pipelines

GitHub Actions: ci-proto.yml, ci-backend.yml (with kind-based e2e), ci-cli.yml, ci-chart.yml.

---

## Chunk 12-16: Rate Limiting, Self-Update, Federation, Dogfood

Same scope as v1 plan chunks 11-14, adapted for PostgreSQL advisory locks instead of Firestore distributed locks, and Valkey pub/sub instead of Firestore listeners.

---

## Dependencies

```
Chunk 1 (scaffolding)
  -> Chunk 2 (backend healthz + ListSkills)
       -> Chunk 3 (CLI explore)
            -> Chunk 4 (PostgreSQL store)
                 -> Chunk 5 (auth)
                      -> Chunk 6 (publish/OCI)
                           -> Chunk 7 (install/OCI pull)
                                -> Chunk 8 (search)

Chunk 4 (PostgreSQL)
  -> Chunk 9 (Valkey cache)

Chunk 2 (backend) + Chunk 9 (Valkey) + Chunk 10 (Helm)
  -> Chunk 11 (CI/CD)

Chunk 5 (auth) + Chunk 4 (PostgreSQL)
  -> Chunk 15 (federation)

Independent after Chunk 2:
  Chunk 12 (rate limiting)
  Chunk 13 (self-update, after Chunk 11)
```
