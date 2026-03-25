# skillsctl - Skill Registry CLI

You have access to `skillsctl`, a CLI tool for discovering, installing, and publishing Claude Code skills from a shared registry.

## Checking if skillsctl is installed

Before using any commands, check if skillsctl is available:

```bash
which skillsctl
```

If not installed, tell the user to install it. Recommend the method that fits their setup:

**Homebrew (macOS/Linux):**
```bash
brew tap nebari-dev/tap
brew install skillsctl
```

**Shell script (macOS/Linux):**
```bash
curl -sSL https://raw.githubusercontent.com/nebari-dev/skillsctl/main/install.sh | bash
```

**Go:**
```bash
go install github.com/nebari-dev/skillsctl/cli@latest
```

## First-time setup

If the user hasn't configured skillsctl yet, run the interactive setup:

```bash
skillsctl config init
```

This prompts for:
- **API URL** - the registry server (the user's org provides this)
- **Skills directory** - where skills are installed (defaults to `~/.claude/skills`)

For non-interactive setup:

```bash
skillsctl config set api_url https://skillsctl.example.com
```

## Authentication

For registries that require authentication:

```bash
skillsctl auth login
```

This opens a browser-based OIDC device flow. The user visits a URL, enters a code, and the CLI caches the token locally. No manual token management needed.

Check auth status:

```bash
skillsctl auth status
```

If a command fails with "Not authenticated", prompt the user to run `skillsctl auth login`.

## Discovering skills

Browse available skills:

```bash
# List all skills
skillsctl explore

# Filter by tag
skillsctl explore --tag go --tag testing

# Filter by source (internal or external)
skillsctl explore --source internal
```

Show details for a specific skill:

```bash
skillsctl explore show <name>

# Include the full skill content
skillsctl explore show <name> --verbose
```

## Installing skills

Install a skill (latest version):

```bash
skillsctl install <name>
```

Install a specific version:

```bash
skillsctl install <name>@<version>
```

Install with digest verification:

```bash
skillsctl install <name>@<version> --digest sha256:<hash>
```

Skills are installed to the configured skills directory (default `~/.claude/skills/<name>.md`). After installing, the skill is immediately available in Claude Code sessions.

## Publishing skills

Publish a skill from a local file:

```bash
skillsctl publish \
  --name my-skill \
  --version 1.0.0 \
  --description "What this skill does" \
  --file ./my-skill.md \
  --tag go \
  --tag testing \
  --changelog "Initial release"
```

Required flags: `--name`, `--version`, `--description`, `--file`
Optional flags: `--tag` (repeatable), `--changelog`

The file must be under 1MB. Skill names are lowercase alphanumeric with hyphens (2-64 chars). Versions must be valid semver.

Publishing a version is permanent - you cannot overwrite an existing version. Publish a new version instead.

## Common workflows

### "I want to find a skill for X"

```bash
skillsctl explore --tag X
skillsctl explore show <interesting-skill> --verbose
skillsctl install <interesting-skill>
```

### "I want to share a skill I wrote"

```bash
# Make sure you're authenticated
skillsctl auth login

# Publish it
skillsctl publish --name my-skill --version 1.0.0 --description "Does X" --file ./skill.md --tag relevant-tag
```

### "I want to update an installed skill"

```bash
# Check what's available
skillsctl explore show <name>

# Install the new version (overwrites the old one)
skillsctl install <name>@<new-version>
```

### "A command says I'm not authenticated"

```bash
skillsctl auth status
# If expired or not logged in:
skillsctl auth login
```

## Troubleshooting

- **"Cannot reach server"** - check `skillsctl config get api_url` and verify the server is reachable
- **"Not authenticated"** - run `skillsctl auth login`
- **"Server does not require authentication"** - the registry is running in dev mode, no login needed
- **"Version X already exists"** - versions are immutable, bump the version number
- **"Permission denied"** - you're not the owner of this skill (only the original publisher can update it)
