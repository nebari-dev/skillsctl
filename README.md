# skillctl

A CLI tool and backend registry for discovering, installing, and publishing [Claude Code](https://claude.ai/code) skills.

## Install

### Homebrew (macOS/Linux)

    brew tap nebari-dev/tap
    brew install skillctl

### Shell script (macOS/Linux)

    curl -sSL https://raw.githubusercontent.com/nebari-dev/skillctl/main/install.sh | bash

### Go

    go install github.com/nebari-dev/skillctl/cli@latest

### From source

    git clone https://github.com/nebari-dev/skillctl.git
    cd skillctl && make build-cli

## Quick Start

Start the backend locally (no external dependencies):

    go run ./backend/cmd/server
    # Server starts on :8080, auth disabled (dev mode)
    # Health check: curl localhost:8080/healthz

Use the CLI:

    skillctl config init                  # first-time setup
    skillctl explore                      # browse skills
    skillctl explore show <name>          # skill details
    skillctl install <name>               # install latest version
    skillctl install <name>@1.0.0         # install specific version
    skillctl publish --name my-skill \
      --version 1.0.0 \
      --description "My skill" \
      --file ./skill.md                   # publish a skill
    skillctl auth login                   # authenticate (production servers)

## Architecture

| Component | Technology |
|-----------|-----------|
| API | ConnectRPC (gRPC-compatible, serves JSON/HTTP) |
| Database | SQLite (modernc.org/sqlite, pure Go, WAL mode) |
| Skill storage | Content stored as BLOB in SQLite |
| Auth | Generic OIDC token validation (works with Keycloak, Okta, Dex) |
| CLI auth | RFC 8628 device flow, zero-config (discovers settings from server) |

## Project Structure

    .
    ├── proto/              # Protobuf service definitions
    ├── gen/                # Generated code (committed, never hand-edited)
    ├── backend/            # ConnectRPC server
    │   ├── cmd/server/     # Entrypoint
    │   └── internal/       # Server internals (auth, registry, store)
    ├── cli/                # Cobra CLI
    │   ├── cmd/            # CLI commands
    │   └── internal/       # CLI internals (api client, auth, config)
    ├── skills/             # Dogfood skills shipped with the project
    └── docs/               # Documentation

## Development

### Prerequisites

- Go 1.25+
- [buf](https://buf.build/docs/installation) (protobuf tooling)
- [golangci-lint](https://golangci-lint.run/welcome/install/)

### Commands

    make test             # all tests with race detector
    make test-backend     # backend tests only
    make test-cli         # CLI tests only
    make lint             # golangci-lint
    make proto            # buf lint + generate
    make build-cli        # builds ./skillctl
    make build-backend    # builds ./skillctl-server

### Proto Changes

1. Edit `.proto` files in `proto/skillctl/v1/`
2. Run `make proto` to lint and regenerate
3. Verify with `git diff --exit-code gen/` (CI checks for drift)

### Database Migrations

Migrations use [goose](https://github.com/pressly/goose) and live in `backend/internal/store/sqlite/migrations/`. The server runs migrations automatically on startup.

## License

See [LICENSE](LICENSE).
