---
title: "Server"
weight: 30
bookCollapseSection: true
---

# Server

The SkillsCtl server is the registry backend. It stores skills, enforces ownership, and serves the ConnectRPC API that the CLI talks to.

## Deployment options

| Guide | When to use |
|-------|-------------|
| [Local and Docker]({{< relref "/server/local" >}}) | Development, evaluation, single-machine deployments |
| [Kubernetes (Helm)]({{< relref "/server/kubernetes" >}}) | Production Kubernetes clusters |
| [Nebari integration]({{< relref "/server/nebari" >}}) | Clusters running the nebari-operator |
| [Configuration reference]({{< relref "/server/configuration" >}}) | All environment variables and OIDC setup |

## How it works

The server is a single Go binary. It uses SQLite for persistence (WAL mode, pure Go driver - no native dependencies). Skills and their content are stored in the database. The server exposes:

- `GET /healthz` - health check, no auth required
- `GET /auth/config` - returns OIDC settings so the CLI can self-configure, no auth required
- `POST /skillsctl.v1.RegistryService/*` - ConnectRPC handlers for skill operations

When started without OIDC environment variables, the server runs in dev mode: auth is disabled and a default identity is injected for ownership tracking. This is safe for local development but should not be used in production.
