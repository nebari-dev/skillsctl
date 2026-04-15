---
title: "publish"
weight: 30
---

# publish

Upload a skill directory to the registry. Requires authentication unless the server is running in dev mode.

## Synopsis

```
skillsctl publish --name NAME --version VERSION --description DESC --dir PATH \
  [--tag TAG ...] [--changelog TEXT]
```

## Flags

### Required

| Flag | Description |
|------|-------------|
| `--name NAME` | Skill name. Must be unique in the registry. Use lowercase letters, numbers, and hyphens. |
| `--version VERSION` | Semantic version string, e.g. `1.0.0`. |
| `--description DESC` | Short description of what the skill does. |
| `--dir PATH` | Path to the skill directory to upload. Must contain `SKILL.md` at its root. The directory is packaged into a deterministic tar.gz before upload. |

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
  --dir ./git-conventional
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
  --dir ./git-conventional \
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

**Error: missing SKILL.md at root of directory**
The directory passed to `--dir` must contain a `SKILL.md` file at its top level.

**Error: packed size exceeds limit**
The tar.gz exceeds the server's `MaxPackedBytes` limit (default 5 MB). Remove large files or split the skill into multiple skills.

**Error: file too large**
An individual file in the directory exceeds the server's `MaxFileBytes` limit (default 1 MB).

**Error: unauthenticated**
The server requires a valid token. Run `skillsctl auth login` and try again.

**Error: permission denied: skill owned by another user**
A different account published earlier versions of this skill. You cannot publish new versions for a skill you do not own.
