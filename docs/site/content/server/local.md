---
title: "Local and Docker"
weight: 10
---

# Local and Docker

This page covers running the SkillsCtl server on a local machine, either from source or with Docker.

## Running from source

You need Go 1.21 or later. Clone the repository if you haven't already:

```bash
git clone https://github.com/nebari-dev/skillsctl.git
cd SkillsCtl
```

Start the server:

```bash
go run ./backend/cmd/server
```

Or build and run the binary:

```bash
CGO_ENABLED=0 go build -ldflags="-s -w" -o skillsctl-server ./backend/cmd/server
./skillsctl-server
```

Expected startup output:

```
2026/03/18 10:00:00 auth disabled (no OIDC_ISSUER_URL)
2026/03/18 10:00:00 starting server on :8080 (db: SkillsCtl.db)
```

The server listens on `:8080` and creates `skillsctl.db` in the current directory.

Verify it is running:

```bash
curl localhost:8080/healthz
```

Expected response: `ok`

## Dev mode

When `OIDC_ISSUER_URL` is not set, the server starts in dev mode:

- Authentication is disabled - all requests are accepted
- A default user identity is injected so ownership tracking works
- No credentials are needed to publish or install skills

Dev mode is useful for local development and evaluation. Do not run dev mode in production.

## Configuring the port and database path

Set environment variables before starting:

```bash
PORT=9090 DB_PATH=/var/data/skillsctl.db go run ./backend/cmd/server
```

Or export them:

```bash
export PORT=9090
export DB_PATH=/var/data/skillsctl.db
go run ./backend/cmd/server
```

See [Configuration reference]({{< relref "/server/configuration" >}}) for all available variables.

## Running with Docker

Pull the image:

```bash
docker pull ghcr.io/nebari-dev/skillsctl-backend:latest
```

Run with a local volume for the database:

```bash
docker run -d \
  --name SkillsCtl \
  -p 8080:8080 \
  -v skillsctl-data:/data \
  -e DB_PATH=/data/skillsctl.db \
  ghcr.io/nebari-dev/skillsctl-backend:latest
```

The container runs as UID 65534 (non-root). Mount the volume at a path writable by that user, or pre-create the directory with appropriate permissions.

Verify:

```bash
curl localhost:8080/healthz
```

## Running with Docker Compose

A minimal `docker-compose.yml` for local use:

```yaml
services:
  skillsctl:
    image: ghcr.io/nebari-dev/skillsctl-backend:latest
    ports:
      - "8080:8080"
    volumes:
      - skillsctl-data:/data
    environment:
      DB_PATH: /data/skillsctl.db

volumes:
  skillsctl-data:
```

Start it:

```bash
docker compose up -d
```

## Connecting the CLI

Once the server is running, configure the CLI to point at it:

```bash
SkillsCtl config init
```

Accept the default `http://localhost:8080` when prompted. If the server is on a different host or port, enter that URL instead.

## Next steps

- [Kubernetes deployment]({{< relref "/server/kubernetes" >}}) - Helm chart for production clusters
- [Configuration reference]({{< relref "/server/configuration" >}}) - all environment variables
