# CI/CD Design Spec

## Goal

Automated testing, linting, and release pipeline for skillsctl. CLI binaries published to GitHub Releases via GoReleaser with Homebrew tap support, backend Docker image published to ghcr.io.

## Versioning

Single version for both CLI and backend, derived from git tags (e.g., `v0.1.0`). GoReleaser injects the version via ldflags. The existing `VERSION` file is for local dev builds only - CI ignores it.

## GoReleaser Config (`.goreleaser.yml`)

Builds the CLI binary for:
- linux/amd64, linux/arm64
- darwin/amd64, darwin/arm64
- windows/amd64

Binary name: `skillsctl`. Entry point: `./cli` (the `main:` field in GoReleaser uses the package directory, not the file path).

Build flags:
```
CGO_ENABLED=0
ldflags: -s -w -X github.com/nebari-dev/skillsctl/cli/cmd.version={{.Version}} -X github.com/nebari-dev/skillsctl/cli/cmd.commit={{.ShortCommit}}
```

Note: The ldflags target is `cli/cmd.version`, not `main.version`, because the version variable lives in the `cmd` package where `rootCmd.Version` is set. This is the standard pattern when using Cobra - the `cmd` package owns the root command and its version string.

Archives use `.tar.gz` for linux/darwin, `.zip` for windows. Checksum file (`checksums.txt`) generated with SHA-256.

Changelog auto-generated from commit messages since the last tag, grouped by conventional commit type (feat, fix, etc.).

### Homebrew

GoReleaser generates a Homebrew formula and pushes it to `nebari-dev/homebrew-tap` (already created at https://github.com/nebari-dev/homebrew-tap).

```yaml
brews:
  - repository:
      owner: nebari-dev
      name: homebrew-tap  # actual repo name; Homebrew strips "homebrew-" prefix
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    homepage: https://github.com/nebari-dev/skillsctl
    description: CLI for discovering, installing, and publishing Claude Code skills
    install: |
      bin.install "skillsctl"
    test: |
      system "#{bin}/skillsctl", "--version"
```

Users install with `brew tap nebari-dev/tap` (Homebrew auto-resolves `tap` to the `homebrew-tap` repo) then `brew install skillsctl`.

Requires a `HOMEBREW_TAP_TOKEN` GitHub Actions secret - a PAT with `repo` scope for the homebrew-tap repo.

GoReleaser does NOT build the Docker image - that's handled separately in the GitHub Action to keep concerns separate.

## Backend Dockerfile (`backend/Dockerfile`)

Multi-stage build:

```
Stage 1: golang:1.24-alpine
- Copy go.mod, go.sum, download deps
- Copy source, build with CGO_ENABLED=0, ldflags="-s -w"
- Output: /skillsctl-server

Stage 2: scratch
- Copy binary from builder
- Copy /etc/ssl/certs for HTTPS (OIDC discovery needs TLS)
- USER 65534:65534 (nobody - don't run as root)
- EXPOSE 8080
- ENTRYPOINT ["/skillsctl-server"]
```

The image is minimal (just the static binary + CA certs). No shell, no package manager. Runs as non-root user (UID 65534/nobody).

## GitHub Actions

### `ci.yml` - Continuous Integration

Triggers: push to any branch, pull requests to main.

Uses `actions/setup-go` with `cache: true` for Go module and build cache.

Jobs (all run in parallel):

**test:**
- `ubuntu-latest`, Go 1.24
- `go test ./... -race -coverprofile=coverage.out`

**lint:**
- `ubuntu-latest`
- `golangci-lint` via the official action

**build:**
- Compile-check both binaries (no output flag, just verifies they compile):
  - `CGO_ENABLED=0 go build ./cli`
  - `CGO_ENABLED=0 go build ./backend/cmd/server`

**proto-check:**
- Install buf
- `cd proto && buf lint && buf generate`
- `git diff --exit-code gen/` - fails if generated code is stale
- Generated code must be committed; CI only verifies it's not stale, it does not commit on your behalf

### `release.yml` - Release Pipeline

Triggers: push of tags matching `v*`.

Jobs:

**test:**
- Same as ci.yml test job (intentionally duplicated rather than using reusable workflows, to keep the dependency chain simple)

**release-cli:**
- Needs: test
- Uses `goreleaser/goreleaser-action`
- Publishes CLI binaries to GitHub Releases
- Pushes Homebrew formula to `nebari-dev/homebrew-tap`
- Requires `GITHUB_TOKEN` (automatic) and `HOMEBREW_TAP_TOKEN` (secret)
- Permissions: `contents: write` (for GitHub Releases)

**release-backend:**
- Needs: test
- Permissions: `contents: read`, `packages: write` (required for ghcr.io push)
- Log in to ghcr.io with `GITHUB_TOKEN`
- Build Docker image with tags:
  - `ghcr.io/nebari-dev/skillsctl-backend:<tag>` (e.g., `v0.1.0`)
  - `ghcr.io/nebari-dev/skillsctl-backend:latest`
- Push both tags

## install.sh

A shell script at the repo root for `curl -sSL ... | bash` installation. Linux and macOS only.

Behavior:
1. Detect OS (`uname -s` -> linux/darwin). Windows users get a message to download from GitHub Releases or use `go install`.
2. Detect arch (`uname -m` -> x86_64->amd64, aarch64/arm64->arm64)
3. Fetch the latest release tag from GitHub API
4. Download the archive and checksums from GitHub Releases
5. Verify SHA-256 checksum
6. Extract binary
7. Install to `/usr/local/bin` if writable, otherwise `~/.local/bin` (create if needed, warn to add to PATH)
8. Print version: `skillsctl --version`

Error handling:
- Windows/unsupported OS: "Windows detected. Download from https://github.com/nebari-dev/skillsctl/releases or run: go install github.com/nebari-dev/skillsctl/cli@latest"
- Unsupported arch: print error and exit
- Download failure: print error with URL attempted
- Checksum mismatch: print error, delete downloaded file, exit 1
- No write permission: print error suggesting `sudo`

Dependencies: `curl`, `sha256sum` (or `shasum -a 256` on macOS), `tar`, `uname`.

## CLI Version Flag

Add `--version` flag to the root command. The version variable lives in `cli/cmd` (not `main`) so GoReleaser can inject it via ldflags targeting the correct package:

```go
// cli/cmd/root.go
var version = "dev"
var commit = ""

// In NewRootCmd():
rootCmd.Version = version
```

GoReleaser ldflags:
```
-X github.com/nebari-dev/skillsctl/cli/cmd.version={{.Version}}
-X github.com/nebari-dev/skillsctl/cli/cmd.commit={{.ShortCommit}}
```

Local builds show "dev". Tagged releases show the version.

## Documentation Updates

### README.md

Add installation section:

```markdown
## Install

### Homebrew (macOS/Linux)

    brew tap nebari-dev/tap
    brew install skillsctl

### Shell script

    curl -sSL https://raw.githubusercontent.com/nebari-dev/skillsctl/main/install.sh | bash

### Go

    go install github.com/nebari-dev/skillsctl/cli@latest

### From source

    git clone https://github.com/nebari-dev/skillsctl.git
    cd skillsctl && make build-cli
```

### skills/skillsctl-usage.md

Update the installation section to list Homebrew as primary, curl|bash as secondary, `go install` as fallback.

## Required GitHub Secrets

| Secret | Purpose | Scope |
|--------|---------|-------|
| `HOMEBREW_TAP_TOKEN` | Push formula to homebrew-tap repo | PAT with `repo` scope for `nebari-dev/homebrew-tap` |

`GITHUB_TOKEN` is automatic and covers GitHub Releases and ghcr.io.

## Files to Create/Modify

### New files:
- `.goreleaser.yml` - GoReleaser config with Homebrew
- `backend/Dockerfile` - multi-stage Docker build
- `.github/workflows/ci.yml` - test, lint, build, proto check
- `.github/workflows/release.yml` - GoReleaser + Docker release
- `install.sh` - curl|bash installer

### Modified files:
- `cli/cmd/root.go` - add version/commit vars, wire rootCmd.Version
- `README.md` - add installation instructions
- `skills/skillsctl-usage.md` - update install section with Homebrew

## Non-goals

- Helm chart (separate chunk)
- Artifact signing with cosign (roadmap)
- Release branches (single main branch for now)
- Automatic version bumping (manual tag push)
- Windows installer (Scoop/Chocolatey - not needed yet)
