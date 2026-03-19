---
title: "explore"
weight: 10
---

# explore

Browse skills available in the registry. With no arguments, lists all skills. Use `explore show` to see details about a specific skill.

## Synopsis

```
SkillsCtl explore [--tag TAG] [--source all|internal|external]
SkillsCtl explore show NAME [--verbose]
```

## Subcommands

| Subcommand | Description |
|------------|-------------|
| `show NAME` | Show details for a named skill |

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--tag TAG` | | Filter by tag. Can be specified multiple times. |
| `--source VALUE` | `all` | Filter by source: `all`, `internal`, or `external`. |
| `--verbose` | `false` | (show only) Print the full skill content. |

## Examples

List all skills:

```bash
SkillsCtl explore
```

```
NAME                    VERSION   SOURCE     DESCRIPTION
git-conventional        1.2.0     internal   Enforce conventional commit messages
python-docstrings       0.4.1     internal   Generate Google-style Python docstrings
k8s-yaml-lint           0.1.0     external   Lint Kubernetes YAML manifests
```

Filter by tag:

```bash
SkillsCtl explore --tag git
```

```
NAME                    VERSION   SOURCE     DESCRIPTION
git-conventional        1.2.0     internal   Enforce conventional commit messages
```

Filter by source:

```bash
SkillsCtl explore --source external
```

```
NAME                    VERSION   SOURCE     DESCRIPTION
k8s-yaml-lint           0.1.0     external   Lint Kubernetes YAML manifests
```

Show details for a skill:

```bash
SkillsCtl explore show git-conventional
```

```
Name:        git-conventional
Description: Enforce conventional commit messages
Owner:       alice
Tags:        git, commits
Version:     1.2.0
Installs:    85
Source:      internal

Versions:
  1.2.0
  1.1.0
  1.0.0
```

Show details including the full skill content:

```bash
SkillsCtl explore show git-conventional --verbose
```

The output adds a `--- Content ---` section after the version list with the raw skill markdown.

## Common errors

**No skills found.**
The registry is empty, or no skills match the given filters. Try `skillsctl explore` without filters to confirm connectivity.

**Error: failed to connect to registry: ...**
The CLI cannot reach the server. Check `skillsctl config get api_url` and verify the server is running.
