---
title: "Publishing skills"
weight: 50
---

# Publishing skills

Skills are Markdown files that provide Claude Code with specialized instructions, context, or behavior. Publishing a skill makes it available to everyone with access to your registry.

## Write a skill file

A skill is a plain Markdown file. The content is used directly as a Claude Code instruction set - write it as you would any CLAUDE.md or skill prompt.

Here is an example skill that helps write SQL queries:

```markdown
# SQL Query Writer

You help write and optimize SQL queries for PostgreSQL databases.

## Guidelines

- Always use explicit column names - never SELECT *
- Add comments explaining complex joins or subqueries
- Suggest indexes when a query would benefit from them
- Use CTEs (WITH clauses) for readability when queries have multiple steps
- Format queries with consistent indentation: 2 spaces per level

## Output format

When writing a query, provide:
1. The query itself, formatted and commented
2. A brief explanation of the approach
3. Any index recommendations, if applicable

## Examples

User: "Get all users who placed an order in the last 30 days"

```sql
-- Users with recent order activity
SELECT DISTINCT
  u.id,
  u.email,
  u.created_at
FROM users u
INNER JOIN orders o ON o.user_id = u.id
WHERE o.created_at >= NOW() - INTERVAL '30 days'
ORDER BY u.id;
```
```

Save this as `sql-writer.md`.

## Publish the skill

Use `skillsctl publish` with the required flags:

```bash
SkillsCtl publish \
  --name sql-writer \
  --version 1.0.0 \
  --description "Helps write and optimize PostgreSQL queries" \
  --file sql-writer.md \
  --tag sql \
  --tag database \
  --changelog "initial release"
```

Flags:

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | yes | Skill identifier (lowercase, hyphens allowed) |
| `--version` | yes | Semantic version (e.g. `1.0.0`) |
| `--description` | yes | Short description shown in `explore` |
| `--file` | yes | Path to the skill Markdown file |
| `--tag` | no | Tag for filtering (repeatable) |
| `--changelog` | no | Release notes for this version |

On success:

```
Published sql-writer@1.0.0 (sha256:c4e8f2a1b3d567890abcdef1234567890abcdef1234567890abcdef1234567890)
```

The digest is printed inline. Save it if you want to share a verified install command with your team.

## Versions are immutable

Once published, a version cannot be overwritten. If you try:

```bash
SkillsCtl publish \
  --name sql-writer \
  --version 1.0.0 \
  --description "..." \
  --file sql-writer.md
```

```
Error: version already exists: sql-writer@1.0.0
```

To release an update, increment the version:

```bash
SkillsCtl publish \
  --name sql-writer \
  --version 1.1.0 \
  --description "Helps write and optimize PostgreSQL queries" \
  --file sql-writer.md \
  --tag sql \
  --tag database \
  --changelog "add CTE guidance and index recommendations"
```

Version immutability ensures that anyone who installed `sql-writer@1.0.0` with a pinned version or digest will always get exactly what was published, even after newer versions are released.

## Authentication

On a server configured with OIDC, you must be logged in before publishing:

```bash
SkillsCtl auth login
SkillsCtl publish --name ... --version ... --description ... --file ...
```

Run `skillsctl auth login` and follow the device flow prompt in your browser. The CLI discovers the OIDC issuer URL automatically from the server.

On a dev mode server (no OIDC), publish works without authentication.

## Ownership

The skill is owned by the OIDC subject used when publishing. Only the original owner can publish new versions of the same skill name. Ownership is based on the immutable OIDC subject, not email, so it survives email address changes.

## Next steps

- [Configuration]({{< relref "/getting-started/configuration" >}}) - set a non-default server URL
- [Exploring skills]({{< relref "/getting-started/exploring" >}}) - verify your skill appears in the registry
