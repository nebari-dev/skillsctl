---
title: "Exploring skills"
weight: 30
---

# Exploring skills

The `skillsctl explore` command lets you browse and search skills in the registry.

## List all skills

```bash
skillsctl explore
```

Output is a table showing key metadata for each skill:

```
SOURCE    NAME              OWNER           TAGS                        INSTALLS  VERSION
internal  code-review       alice@corp.dev  review,quality              142       1.3.0
internal  git-commit        bob@corp.dev    git,workflow                98        2.0.1
internal  sql-optimizer     alice@corp.dev  sql,database,performance    67        1.0.2
internal  openapi-writer    carol@corp.dev  api,docs,openapi            55        1.1.0
internal  test-generator    bob@corp.dev    testing,tdd                 41        0.9.0
internal  pr-description    carol@corp.dev  git,github,workflow         38        1.2.0
internal  dockerfile-linter dave@corp.dev   docker,devops               22        1.0.0
```

Columns:

- **SOURCE** - where the skill came from (`internal` for your registry, `external` for federated)
- **NAME** - skill identifier used with `install` and `explore show`
- **OWNER** - publisher's identity (OIDC subject or email)
- **TAGS** - comma-separated labels for filtering
- **INSTALLS** - total install count across all versions
- **VERSION** - latest published version

## Filter by tag

Use `--tag` to narrow results to skills with a specific tag. You can repeat the flag to require multiple tags:

```bash
skillsctl explore --tag git
```

```
SOURCE    NAME            OWNER           TAGS              INSTALLS  VERSION
internal  git-commit      bob@corp.dev    git,workflow      98        2.0.1
internal  pr-description  carol@corp.dev  git,github,workflow  38     1.2.0
```

```bash
skillsctl explore --tag git --tag workflow
```

```
SOURCE    NAME            OWNER           TAGS                 INSTALLS  VERSION
internal  git-commit      bob@corp.dev    git,workflow         98        2.0.1
internal  pr-description  carol@corp.dev  git,github,workflow  38        1.2.0
```

## View skill details

Use `explore show` to see full metadata for a skill, including its version history:

```bash
skillsctl explore show git-commit
```

```
Name:        git-commit
Description: Helps write clear, conventional commit messages following team standards
Owner:       bob@corp.dev
Tags:        git, workflow
Version:     2.0.1
Installs:    98
Source:      internal

Versions:
  2.0.1
  2.0.0
  1.1.0
  1.0.0
```

## View skill content

Add `--verbose` to include the full skill content in the output:

```bash
skillsctl explore show git-commit --verbose
```

```
Name:        git-commit
Description: Helps write clear, conventional commit messages following team standards
Owner:       bob@corp.dev
Tags:        git, workflow
Version:     2.0.1
Installs:    98
Source:      internal

Versions:
  2.0.1
  2.0.0
  1.1.0
  1.0.0

--- Content ---
You are helping write a git commit message. Follow conventional commits format:

<type>(<scope>): <subject>

Types: feat, fix, docs, style, refactor, test, chore
...
```

## Next steps

- [Install a skill]({{< relref "/getting-started/installing" >}}) - add a skill to your Claude Code workspace
- [Publish a skill]({{< relref "/getting-started/publishing" >}}) - share your own skills
