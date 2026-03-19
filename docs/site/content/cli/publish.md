---
title: "publish"
weight: 30
---

# publish

Upload a skill file to the registry. Requires authentication unless the server is running in dev mode.

## Synopsis

```
skillsctl publish --name NAME --version VERSION --description DESC --file PATH \
  [--tag TAG ...] [--changelog TEXT]
```

## Flags

### Required

| Flag | Description |
|------|-------------|
| `--name NAME` | Skill name. Must be unique in the registry. Use lowercase letters, numbers, and hyphens. |
| `--version VERSION` | Semantic version string, e.g. `1.0.0`. |
| `--description DESC` | Short description of what the skill does. |
| `--file PATH` | Path to the skill file to upload. Maximum size: 1 MB. |

### Optional

| Flag | Description |
|------|-------------|
| `--tag TAG` | Tag to apply to the skill. Can be repeated to add multiple tags. |
| `--changelog TEXT` | Release notes for this version. |

## Examples

Publish a new skill:

```bash
skillsctl publish \
  --name git-conventional \
  --version 1.0.0 \
  --description "Enforce conventional commit messages" \
  --file ./git-conventional.md
```

```
Published git-conventional@1.0.0
```

Publish with tags and a changelog:

```bash
skillsctl publish \
  --name git-conventional \
  --version 1.1.0 \
  --description "Enforce conventional commit messages" \
  --file ./git-conventional.md \
  --tag git \
  --tag commits \
  --changelog "Add breaking change detection"
```

```
Published git-conventional@1.1.0
```

## Version immutability

Once a version is published, it cannot be overwritten. Publishing `git-conventional@1.0.0` a second time returns an error. Increment the version number to publish an update.

## Authentication

On servers with OIDC configured, you must be logged in before publishing. Run `skillsctl auth login` first. Ownership of a skill is tied to your OIDC subject (not your email), so the same account must be used for all versions of a skill.

## Common errors

**Error: version "X" already exists for skill "NAME"**
That version has already been published. Choose a new version number.

**Error: file too large (max 1 MB)**
The skill file exceeds the 1 MB limit. Split the skill or reduce its size.

**Error: unauthenticated**
The server requires a valid token. Run `skillsctl auth login` and try again.

**Error: permission denied: skill owned by another user**
A different account published earlier versions of this skill. You cannot publish new versions for a skill you do not own.
