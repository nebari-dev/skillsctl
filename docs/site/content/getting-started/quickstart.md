---
title: "Quick Start"
weight: 20
---

# Quick Start

This guide walks you through running a local skillsctl registry server and connecting the CLI to it. The whole process takes about five minutes.

## Prerequisites

- Go 1.21 or later (to run the server from source)
- skillsctl CLI [installed]({{< relref "/getting-started/installation" >}})

## Start a local server

Clone the repository if you haven't already:

```bash
git clone https://github.com/nebari-dev/skillsctl.git
cd skillsctl
```

Start the server:

```bash
go run ./backend/cmd/server
```

You should see output like:

```
2026/03/18 10:00:00 auth disabled (no OIDC_ISSUER_URL)
2026/03/18 10:00:00 starting server on :8080 (db: skillsctl.db)
```

The server listens on port 8080. Leave this terminal open.

## Verify the server is running

In a new terminal, hit the health endpoint:

```bash
curl localhost:8080/healthz
```

Expected response:

```
ok
```

## Configure the CLI

Run the interactive setup:

```bash
skillsctl config init
```

The prompts ask for a server URL and skills directory. Accept the defaults by pressing Enter:

```
No configuration found. Let's set up skillsctl.

API URL [http://localhost:8080]:
Skills directory [/home/you/.claude/skills]:

Config saved to /home/you/.config/skillsctl/config.yaml
```

Verify the configuration was saved:

```bash
skillsctl config list
```

```
api_url: http://localhost:8080
```

## Test the connection

List available skills (the local server starts empty):

```bash
skillsctl explore
```

```
No skills found.
```

That's expected - you're connected and ready to publish your first skill.

## Dev mode note

When the server starts without OIDC configuration, it runs in dev mode. In dev mode:

- Authentication is disabled - no login required
- All API operations are allowed without credentials
- A default user identity is injected automatically for ownership tracking

Dev mode is useful for local development and evaluation. For production deployments, configure OIDC. See the [server documentation]({{< relref "/server" >}}) for details.

## Next steps

- [Explore skills]({{< relref "/getting-started/exploring" >}}) - browse a registry with existing skills
- [Publish a skill]({{< relref "/getting-started/publishing" >}}) - add your first skill to the local registry
- [Configuration]({{< relref "/getting-started/configuration" >}}) - set options like a production server URL
