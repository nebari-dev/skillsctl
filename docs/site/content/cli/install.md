---
title: "install"
weight: 20
---

# install

Download a skill from the registry and write it to the local skills directory.

## Synopsis

```
skillsctl install NAME[@VERSION] [--digest sha256:...] [--skills-dir DIR]
```

If `@VERSION` is omitted, the latest published version is installed.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--digest sha256:HASH` | | Verify the downloaded content matches this digest before writing. |
| `--skills-dir DIR` | `~/.claude/skills` | Directory to install the skill file into. Overrides `skills_dir` from config. |

## Examples

Install the latest version of a skill:

```bash
skillsctl install git-conventional
```

```
Installed git-conventional@1.2.0 to /home/you/.claude/skills/git-conventional.md
```

Install a specific version:

```bash
skillsctl install git-conventional@1.1.0
```

```
Installed git-conventional@1.1.0 to /home/you/.claude/skills/git-conventional.md
```

Install with digest verification:

```bash
skillsctl install git-conventional --digest sha256:abc123...
```

If the digest does not match, the install is aborted and no file is written.

Install to a custom directory:

```bash
skillsctl install git-conventional --skills-dir /tmp/test-skills
```

```
Installed git-conventional@1.2.0 to /tmp/test-skills/git-conventional.md
```

## How it works

1. The CLI fetches the skill content from `GetSkillContent` (unauthenticated).
2. If `--digest` is provided, the downloaded content is hashed and compared before writing.
3. The file is written atomically: content is written to a temporary file, then renamed into place.

## Common errors

**Error: skill "NAME" not found**
No skill with that name exists in the registry. Use `skillsctl explore` to list available skills.

**Error: version "X" not found for skill "NAME"**
The requested version does not exist. Use `skillsctl explore show NAME` to list available versions.

**Error: digest mismatch: expected sha256:X, got sha256:Y**
The downloaded content does not match the expected digest. The file was not written. Check that the digest value is correct.

**Error: skills directory does not exist: ...**
The target directory is missing. Create it manually or set a valid path with `skillsctl config set skills_dir PATH`.
