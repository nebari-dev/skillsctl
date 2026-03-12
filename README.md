# skillctl

A Kubernetes-native CLI tool and backend registry for discovering, installing, and publishing [Claude Code](https://claude.ai/code) skills. Supports federated marketplace integration with admin-controlled external skill whitelisting.

## Overview

skillctl provides a centralized, org-controlled registry for Claude Code skills. Developers use the `skillctl` CLI to browse, install, and publish skills. Admins manage which external skill marketplaces are whitelisted for their organization.

**Key features:**
- Browse and search skills with `skillctl explore`
- Install skills to `~/.claude/skills/` with `skillctl install`
- Publish skills as OCI artifacts with `skillctl push`
- Federated marketplace support - admins whitelist external skill sources (GitHub repos, agentskills.io)
- Generic OIDC authentication - works with Keycloak, Okta, Dex, or any compliant provider
- Kubernetes-native deployment via Helm chart

## Architecture

The system is split into two repos:

| Repo | Purpose |
|------|---------|
| `skillctl` (this repo) | OSS tool: CLI, backend server, Helm chart, proto definitions |
| `skillctl-deploy` (private) | Org-specific: ArgoCD manifests, Helm value overrides, Keycloak config |

### Backend Stack

- **API:** ConnectRPC (gRPC-compatible, also serves JSON/HTTP)
- **Database:** PostgreSQL via CloudNativePG operator
- **Cache invalidation:** Valkey pub/sub (not primary storage)
- **Skill archives:** OCI registry (ghcr.io) via oras-go
- **Auth:** Generic OIDC token validation (JWT signature via JWKS, groups claim for admin role)
- **Deployment:** Helm chart published to ghcr.io OCI registry

### CLI

- Go binary built with Cobra + Viper
- Cross-platform: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
- OIDC device flow authentication (RFC 8628)
- OCI artifact pull for skill installation via oras-go

## Prerequisites

- Go 1.25+
- [buf](https://buf.build/docs/installation) (protobuf tooling)
- [golangci-lint](https://golangci-lint.run/welcome/install/) v2+
- [gitleaks](https://github.com/gitleaks/gitleaks) (secrets scanning)
- PostgreSQL 15+ (for backend development, or use Docker)
- [Helm](https://helm.sh/) 3+ (for chart development/testing)

## Quick Start

```bash
# Build
make build-cli        # builds ./skillctl
make build-backend    # builds ./skillctl-server

# Test
make test             # all tests with race detector
make test-backend     # backend tests only
make test-cli         # CLI tests only

# Proto
make proto            # lint + generate

# Lint
make lint             # golangci-lint
```

### Running Locally

Start the backend with in-memory storage (no PostgreSQL required):

```bash
go run ./backend/cmd/server
# Server starts on :8080
# Health check: curl localhost:8080/healthz
```

Use the CLI against the local backend:

```bash
go run ./cli explore
go run ./cli explore show <skill-name>
```

### Running a Single Test

```bash
go test -run TestFunctionName ./path/to/package/...
```

## Project Structure

```
.
├── proto/              # Protobuf service definitions
├── gen/                # Generated code (committed, never hand-edited)
├── backend/            # ConnectRPC server
│   ├── cmd/server/     # Entrypoint
│   └── internal/       # Server internals (auth, registry, store, cache)
├── cli/                # Cobra CLI
│   ├── cmd/            # CLI commands
│   └── internal/       # CLI internals (api client, auth, oci, config)
├── chart/              # Helm chart
├── e2e/                # End-to-end tests
├── skills/             # Dogfood skills shipped with the project
└── docs/               # Documentation
```

## Development

### Proto Changes

1. Edit `.proto` files in `proto/skillctl/v1/`
2. Run `buf lint proto/` to check
3. Run `buf generate proto/` to regenerate
4. Verify with `git diff --exit-code gen/` (CI checks for drift)

### Database Migrations

Migrations use [goose](https://github.com/pressly/goose) and live in `backend/internal/store/migrations/`. The server runs migrations automatically on startup.

### Helm Chart

```bash
helm lint chart/
helm template skillctl chart/ -f chart/ci/test-values.yaml
```

## License

See [LICENSE](LICENSE).
