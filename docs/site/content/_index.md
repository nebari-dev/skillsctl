---
title: "SkillsCtl"
type: docs
---

# SkillsCtl

A CLI tool and registry for discovering, installing, and publishing [Claude Code](https://claude.ai/code) skills.

## Why SkillsCtl?

Claude Code has a built-in marketplace for individual users. It works fine if you're on your own. SkillsCtl is for teams and organizations that need more.

**If you're an individual developer**, the built-in marketplace is probably all you need. Use it.

**If you're a team or org**, you'll run into questions the marketplace doesn't answer:

- **Who published this skill?** SkillsCtl ties every publish to an authenticated OIDC identity. You know exactly who wrote what.
- **Can I trust the content hasn't changed?** Published versions are immutable. Pin a version with a SHA-256 digest and it's the same bytes every time.
- **Who can publish to our registry?** Your OIDC provider controls access. Only authenticated members of your org can publish.
- **Can we run this on our own infrastructure?** SkillsCtl is self-hosted. Your skills stay on your servers, behind your network, under your control.
- **What if someone publishes something malicious?** Ownership enforcement, version immutability, and digest verification form the baseline. LLM-based content scanning and admin approval are on the roadmap.
- **Can we share skills across teams without copy-pasting files?** Federation lets you whitelist external registries. Teams see each other's skills without duplicating anything.

SkillsCtl doesn't replace the built-in marketplace. It adds the controls that organizations need: identity, immutability, access control, and self-hosting. If you don't need those, stick with the marketplace.

## Get started

- [Install SkillsCtl]({{< relref "/getting-started/installation" >}})
- [Quick start guide]({{< relref "/getting-started/quickstart" >}})
- [Explore available skills]({{< relref "/getting-started/exploring" >}})

## Key features

- **Discover** skills with `skillsctl explore` - browse, filter by tag, view details
- **Install** skills with `skillsctl install <name>` - pin versions, verify digests
- **Publish** skills with `skillsctl publish` - version-controlled, owned by publisher
- **Zero-config auth** - CLI discovers OIDC settings from the server automatically
- **Kubernetes-ready** - Helm chart with optional NebariApp CRD integration
