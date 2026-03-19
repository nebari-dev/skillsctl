---
title: "skillsctl"
type: docs
---

# skillsctl

A CLI tool and registry for discovering, installing, and publishing [Claude Code](https://claude.ai/code) skills.

**skillsctl** lets your team share Claude Code skills through a central registry. Developers browse, install, and publish skills with a single command. Admins control access through standard OIDC authentication.

## Get started

- [Install skillsctl]({{< relref "/getting-started/installation" >}})
- [Quick start guide]({{< relref "/getting-started/quickstart" >}})
- [Explore available skills]({{< relref "/getting-started/exploring" >}})

## Key features

- **Discover** skills with `skillsctl explore` - browse, filter by tag, view details
- **Install** skills with `skillsctl install <name>` - pin versions, verify digests
- **Publish** skills with `skillsctl publish` - version-controlled, owned by publisher
- **Zero-config auth** - CLI discovers OIDC settings from the server automatically
- **Kubernetes-ready** - Helm chart with optional NebariApp CRD integration
