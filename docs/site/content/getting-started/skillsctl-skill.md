---
title: "Using the SkillsCtl skill"
weight: 70
---

# Using the SkillsCtl Skill

SkillsCtl ships a skill that teaches Claude Code how to use SkillsCtl itself. This is the bootstrap for the ecosystem: once the skill is installed, Claude Code can help you discover, install, and publish skills without leaving your editor.

## What is the SkillsCtl skill?

The SkillsCtl skill is a Claude Code skill file that contains:

- The full SkillsCtl command reference
- Examples of common workflows (explore, install, publish)
- Guidance on how to find the right skill for a task
- Tips for writing effective skills

When the skill is active, you can ask Claude Code questions like:

- "What skills are available for code review?"
- "Install the git-commit skill"
- "Help me write and publish a skill for writing OpenAPI specs"

Claude Code will call `skillsctl` on your behalf or guide you through the steps interactively.

## Install the SkillsCtl skill

```bash
SkillsCtl install SkillsCtl
```

```
Installed skillsctl@1.0.0 to /home/you/.claude/skills/skillsctl.md
```

That's the entire bootstrap process. Claude Code picks up the new skill file on the next session start.

## Verify the skill is active

Start a new Claude Code session in any project and ask:

```
What skills do I have installed?
```

Claude Code will list the skills it knows about, including the SkillsCtl skill.

## Example: discovering skills through Claude Code

With the SkillsCtl skill installed, you can have a natural conversation:

```
You: I'm about to do a bunch of code reviews. Is there a skill for that?

Claude: I'll check the registry for code review skills.

  $ SkillsCtl explore --tag review

  SOURCE    NAME         OWNER           TAGS              INSTALLS  VERSION
  internal  code-review  alice@corp.dev  review,quality    142       1.3.0

  There's a code-review skill with 142 installs. Want me to install it?

You: Yes, install it.

Claude: $ SkillsCtl install code-review

  Installed code-review@1.3.0 to /home/you/.claude/skills/code-review.md

  The code-review skill is installed. Start a new session and it will be active.
```

## Example: publishing a skill through Claude Code

The SkillsCtl skill also knows how to help you write and publish new skills:

```
You: I want to publish a skill for writing Terraform modules. Help me write it.

Claude: Let's write a Terraform skill. I'll start with a template and we can refine it...

  [Claude drafts the skill content]

  Ready to publish? I'll need a name, version, and description.

You: Name it "terraform-modules", version 1.0.0.

Claude: $ SkillsCtl publish \
    --name terraform-modules \
    --version 1.0.0 \
    --description "Helps write well-structured Terraform modules" \
    --file terraform-modules.md \
    --tag terraform \
    --tag infra \
    --changelog "initial release"

  Published terraform-modules@1.0.0
  Digest: sha256:...
```

## How the bootstrap works

Skills are just Markdown files in `~/.claude/skills/`. The SkillsCtl skill is itself a skill in the registry - there is nothing special about it. This creates a clean bootstrap:

1. Install SkillsCtl (the CLI)
2. Run `skillsctl install skillsctl` (the skill)
3. From now on, Claude Code can manage skills for you

The whole system is self-describing: the tool for managing skills is itself a skill.

## Next steps

- [Explore skills]({{< relref "/getting-started/exploring" >}}) - browse available skills manually
- [Publish a skill]({{< relref "/getting-started/publishing" >}}) - contribute to the ecosystem
