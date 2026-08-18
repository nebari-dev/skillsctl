---
title: "install"
weight: 20
---

# install

Download a skill from the registry and write it to the local skills directory.

## Synopsis

```
skillsctl install NAME[@VERSION] [--digest sha256:...] [--skills-dir DIR | --project[=PATH]]
```

If `@VERSION` is omitted, the latest published version is installed.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--digest sha256:HASH` | | Verify the downloaded content matches this digest before writing. |
| `--skills-dir DIR` | `~/.claude/skills` | Directory to install the skill into. Overrides `skills_dir` from config. |
| `--project[=PATH]` | *(unset)* | Install to `PATH/.claude/skills` instead of the user-level skills directory. When passed without a value, installs into the current working directory. A path must be attached with `=`, not a space. Mutually exclusive with `--skills-dir`. |
| `--force` | `false` | Overwrite an existing multi-file skill directory. Without this flag, install refuses if the target directory already exists. |

## Examples

Install the latest version of a skill:

```bash
skillsctl install git-conventional
```

```
Installed git-conventional@1.2.0 to /home/you/.claude/skills/git-conventional/SKILL.md
```

Install a specific version:

```bash
skillsctl install git-conventional@1.1.0
```

```
Installed git-conventional@1.1.0 to /home/you/.claude/skills/git-conventional/SKILL.md
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
Installed git-conventional@1.2.0 to /tmp/test-skills/git-conventional/SKILL.md
```

Install into the current project (writes to `./.claude/skills/`):

```bash
skillsctl install git-conventional --project
```

```
Installed git-conventional@1.2.0 to ./.claude/skills/git-conventional/SKILL.md
```

Install into a specific project directory:

```bash
skillsctl install git-conventional --project=/path/to/repo
```

```
Installed git-conventional@1.2.0 to /path/to/repo/.claude/skills/git-conventional/SKILL.md
```

## Multi-file skills

Skills published as a directory (containing `SKILL.md` plus subdirectories) are downloaded as a tar.gz and extracted into `<skills-dir>/<name>/`. The directory structure is preserved exactly as it was published.

If the target directory already exists, install refuses with an error. Pass `--force` to overwrite it.

Single-file skills (legacy format) are still installed as a single `SKILL.md` file inside `<skills-dir>/<name>/`.

Installed files are written as `0600` and carry no executable bit, so a bundled script must be run through its interpreter (`bash <skills-dir>/<name>/scripts/run.sh`) rather than directly.

## How it works

1. The CLI fetches the skill content from `GetSkillContent`. Like every RPC, this requires a bearer token; only `/healthz`, `/auth/config`, and `/limits` are reachable without one.
2. If `--digest` is provided, the downloaded content is hashed and compared before writing.
3. The CLI detects whether the content is a tarball (via gzip magic bytes). Multi-file skills are extracted into `<skills-dir>/<name>/`. Single-file skills are written atomically: content is written to a temporary file, then renamed into place.

## Common errors

**Error: skill "NAME" not found**
No skill with that name exists in the registry. Use `skillsctl explore` to list available skills.

**Error: version "X" not found for skill "NAME"**
The requested version does not exist. Use `skillsctl explore show NAME` to list available versions.

**Error: digest mismatch: expected sha256:X, got sha256:Y**
The downloaded content does not match the expected digest. The file was not written. Check that the digest value is correct.

**Error: accepts 1 arg(s), received 2**
Install takes exactly one skill name. Either more than one name was given, or a path was passed to `--project` with a space instead of an `=` - the space-separated form leaves the path as a second positional argument. Use `--project=/path/to/repo`.

**Error: skills directory does not exist: ...**
The target directory is missing. Create it manually or set a valid path with `skillsctl config set skills_dir PATH`.

**Error: skill directory already exists: ...**
A directory for this skill already exists at the install path. Pass `--force` to overwrite it.
