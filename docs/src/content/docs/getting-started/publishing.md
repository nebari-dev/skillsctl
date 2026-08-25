---
title: "Publishing skills"
---

Skills are directories containing a `SKILL.md` file (and any supporting files) that provide Claude Code with specialized instructions, context, or behavior. Publishing a skill makes it available to everyone with access to your registry.

## Write a skill

A skill is a directory with `SKILL.md` at its root. Subdirectories like `scripts/`, `references/`, and `assets/` are packaged and preserved during publish and install.

Here is an example skill that helps write SQL queries. Create the directory structure:

```
sql-writer/
  SKILL.md
```

And put this in `sql-writer/SKILL.md`:

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

## Publish the skill

Use `skillsctl publish` with the required flags:

```bash
skillsctl publish \
  --name sql-writer \
  --version 1.0.0 \
  --description "Helps write and optimize PostgreSQL queries" \
  --dir ./sql-writer \
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
| `--dir` | yes | Path to the skill directory (must contain `SKILL.md` at its root) |
| `--tag` | no | Tag for filtering (repeatable) |
| `--changelog` | no | Release notes for this version |

The directory is packaged into a deterministic tar.gz before upload. The server enforces configurable upload limits (total size, file count, per-file size); `GET /limits` returns the current values.

On success:

```
Published sql-writer@1.0.0 (sha256:c4e8f2a1b3d567890abcdef1234567890abcdef1234567890abcdef1234567890)
```

The digest is printed inline. Save it if you want to share a verified install command with your team.

## Versions are immutable

Once published, a version cannot be overwritten. If you try:

```bash
skillsctl publish \
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
skillsctl publish \
  --name sql-writer \
  --version 1.1.0 \
  --description "Helps write and optimize PostgreSQL queries" \
  --dir ./sql-writer \
  --tag sql \
  --tag database \
  --changelog "add CTE guidance and index recommendations"
```

Version immutability ensures that anyone who installed `sql-writer@1.0.0` with a pinned version or digest will always get exactly what was published, even after newer versions are released.

## Authentication

On a server configured with OIDC, you must be logged in before publishing:

```bash
skillsctl auth login
skillsctl publish --name ... --version ... --description ... --dir ...
```

Run `skillsctl auth login` and follow the device flow prompt in your browser. The CLI discovers the OIDC issuer URL automatically from the server.

On a dev mode server (no OIDC), publish works without authentication.

## Ownership

The skill is owned by the OIDC subject used when publishing. Only the original owner can publish new versions of the same skill name. Ownership is based on the immutable OIDC subject, not email, so it survives email address changes.

## Next steps

- [Configuration](/getting-started/configuration/) - set a non-default server URL
- [Exploring skills](/getting-started/exploring/) - verify your skill appears in the registry
