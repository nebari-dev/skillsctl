---
title: "Writing skills"
weight: 20
---

# Writing skills

A skill is a Markdown file. Writing one well means being specific about what Claude should do, when it should do it, and what it should avoid.

## Basic structure

Skills don't require a fixed structure. A short, direct file often works better than a long one with elaborate sections. A minimal skill:

```markdown
# code-review

When reviewing code, focus on:

- Correctness: does the code do what the comments and tests say it does?
- Error handling: are errors checked and propagated correctly?
- Readability: are names descriptive and logic easy to follow?

Do not comment on style issues covered by the project's linter.
Do not suggest rewrites unless the existing code has a correctness problem.

Format each comment as: `[file:line] issue - suggestion`.
```

This skill defines a scope (what to look for), constraints (what to skip), and an output format. Each of those three elements reduces ambiguity and produces more consistent results.

## What makes a skill effective

**Be specific about scope.** "Help with Terraform" is vague. "When writing Terraform modules, always declare variables with descriptions and validation rules, use `for_each` instead of `count` for resources that users might partially remove, and follow the naming convention `{project}-{env}-{resource}`" is actionable.

**Define output format.** If you expect a particular structure - a checklist, a table, a specific comment prefix - say so. Claude will follow it.

**State what to avoid.** Exclusions are often more valuable than inclusions. "Do not suggest upgrading dependencies unless asked" prevents Claude from adding noise to reviews.

**Keep it focused.** A skill for one task tends to work better than a skill that tries to cover everything. If you find yourself writing a skill with five unrelated sections, consider splitting it into five skills.

**Use imperative language.** "Check for missing error handling" is clearer than "Error handling should be considered."

## The SkillsCtl skill as an example

SkillsCtl ships a skill that teaches Claude Code how to use SkillsCtl itself. Install it with:

```bash
SkillsCtl install SkillsCtl
```

This skill demonstrates a common pattern: teaching Claude the interface of a specific tool. It includes the command reference, common workflows, and examples. Once installed, you can ask Claude to run SkillsCtl commands on your behalf or guide you through publishing a new skill.

The SkillsCtl skill is also a working example of the bootstrap pattern: the tool for managing skills is itself published as a skill in the registry.

## Publishing a skill

Once you have a skill file ready, publish it to the registry:

```bash
SkillsCtl publish \
  --name my-skill \
  --version 1.0.0 \
  --description "Short description of what this skill does" \
  --file my-skill.md \
  --tag review \
  --changelog "initial release"
```

Tags help others discover the skill with `skillsctl explore --tag review`. Choose tags that describe the domain or task rather than the tool.

## Versioning your skill

Use [semantic versioning]({{< relref "/concepts/versioning" >}}): `MAJOR.MINOR.PATCH`.

- Bump `PATCH` for corrections that don't change the skill's behavior from the user's perspective
- Bump `MINOR` for new instructions or examples that are backward compatible
- Bump `MAJOR` for changes that significantly alter what Claude does with the skill active

Versions are immutable once published. If you need to fix a mistake, publish a new version.

## Testing a skill

Install the skill locally and use it for a few sessions. Ask Claude to do the task the skill covers and check whether the output matches your intent. Refine the skill and publish a new version.

There is no automated testing for skills - they are natural language and the only real test is Claude's behavior.

## Next steps

- [Versioning and ownership]({{< relref "/concepts/versioning" >}}) - semver rules, immutability
- [Security]({{< relref "/concepts/security" >}}) - what to consider before publishing a skill others will use
