---
title: "Skills"
weight: 10
---

# Skills

A skill is a directory containing a `SKILL.md` file (and optionally supporting files like scripts, references, and assets) that gives Claude Code instructions for a specific task or domain. When Claude reads a skill, it follows those instructions during the session.

## What a skill contains

A skill is a directory with `SKILL.md` at its root. The Markdown content can include:

- Step-by-step workflows for common tasks
- Project-specific conventions and rules
- Command references and examples
- Constraints Claude should follow

Skills are natural language instructions, not code. Claude interprets them rather than executing them. A well-written skill narrows Claude's behavior for a particular context: a `code-review` skill might specify what to look for, what to skip, and what format to use for comments.

## Where skills are stored

Skills are directories under `~/.claude/skills/` (or the directory you configured with `skillsctl config set skills_dir`). Each skill gets its own subdirectory containing `SKILL.md` and any supporting files.

Installing a skill with `skillsctl install` downloads the content from the registry and writes it to this directory:

```
~/.claude/skills/
  code-review/
    SKILL.md
  terraform-modules/
    SKILL.md
    references/
      examples.md
  skillsctl/
    SKILL.md
```

Deleting the subdirectory removes the skill. There is no install database - the directory contents are the ground truth.

You can also install skills into a specific project with `--project`, which writes to `<project>/.claude/skills/<name>/` instead of the user-level directory. Project skills are only active for Claude Code sessions started in that project.

## How Claude discovers skills

Claude Code reads `~/.claude/skills/` when a session starts. All skill directories present at that point are active for the session. Skills installed after a session starts are not picked up until the next session.

Claude does not load skills selectively. Every skill in the skills directory is loaded on every session start. If you have many skills with overlapping instructions, they may conflict. Keep your installed skills set focused on what you currently need.

## The registry

The SkillsCtl registry stores skills and their versions centrally so they can be shared across a team or organization. Skills in the registry are identified by name (e.g., `code-review`) and version (e.g., `1.3.0`). Each version is immutable once published.

The registry tracks:

- Skill metadata: name, description, tags, owner, install count
- Version history: all published versions, changelogs, SHA-256 digests
- Skill content: the skill directory for each version, packaged as a deterministic tar.gz and stored as a BLOB (legacy single-file skills are stored as raw Markdown)

The CLI is the primary way to interact with the registry. You can also query it directly through the ConnectRPC API if you need to automate skill management.

## Sources

Each skill has a `source` field. The default source is `internal`, meaning it was published directly to this registry. Skills from external registries or imported from outside the system may have a different source. Admins control which external sources are allowed.

## Next steps

- [Writing skills]({{< relref "/concepts/writing-skills" >}}) - how to author a skill
- [Versioning and ownership]({{< relref "/concepts/versioning" >}}) - how versions and ownership work
- [Security]({{< relref "/concepts/security" >}}) - what to consider before installing a skill
