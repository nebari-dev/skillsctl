---
title: "Installing skills"
weight: 40
---

# Installing skills

The `skillsctl install` command downloads a skill from the registry and saves it where Claude Code can find it.

## Install the latest version

```bash
skillsctl install git-commit
```

```
Installed git-commit@2.0.1 to /home/you/.claude/skills/git-commit.md
```

## Install a pinned version

Append `@<version>` to install a specific version:

```bash
skillsctl install git-commit@1.1.0
```

```
Installed git-commit@1.1.0 to /home/you/.claude/skills/git-commit.md
```

Pinning a version is useful in team environments where you want everyone using the same skill revision, or when a newer version changed behavior you rely on.

## Verify with a digest

Use `--digest` to verify the content hash before saving:

```bash
skillsctl install git-commit@2.0.1 --digest sha256:a3f8c1d2e4b567890abcdef1234567890abcdef1234567890abcdef1234567890
```

If the downloaded content does not match the digest, the install is aborted and nothing is written to disk:

```
Error: digest mismatch
  expected: sha256:a3f8c1d2e4b567890abcdef1234567890abcdef1234567890abcdef1234567890
  got:      sha256:b9e1f23a4c678901bcdef2345678901bcdef2345678901bcdef2345678901bcde
```

You can get the expected digest from `skillsctl explore show <name>` or from the skill publisher.

## Where skills are stored

Skills are saved as Markdown files under `~/.claude/skills/`:

```
~/.claude/skills/
  git-commit.md
  code-review.md
  sql-optimizer.md
```

The filename matches the skill name. Installing a skill twice (or installing a newer version) overwrites the existing file atomically - the old file is never left in a partial state.

## Claude Code picks up skills automatically

Claude Code reads all `.md` files from `~/.claude/skills/` on startup. Once a skill is installed, it is available to Claude Code in all your projects without any additional configuration.

To confirm a skill is active, start Claude Code and ask it to describe the skill you just installed.

## Uninstalling a skill

To remove a skill, delete the file from `~/.claude/skills/`:

```bash
rm ~/.claude/skills/git-commit.md
```

There is no `skillsctl uninstall` command - file deletion is sufficient and immediate.

## Next steps

- [Publish a skill]({{< relref "/getting-started/publishing" >}}) - share a skill with your team
- [Bootstrap with the SkillsCtl skill]({{< relref "/getting-started/skillsctl-skill" >}}) - let Claude Code help you discover skills
