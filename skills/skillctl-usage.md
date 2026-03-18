# skillctl - Skill Registry CLI

You have access to `skillctl`, a CLI tool for discovering, installing, and publishing Claude Code skills from a shared registry.

## Checking if skillctl is installed

Before using any commands, check if skillctl is available:

```bash
which skillctl
```

If not installed, tell the user to install it. Recommend the method that fits their setup:

**Homebrew (macOS/Linux):**
```bash
brew tap nebari-dev/tap
brew install skillctl
```

**Shell script (macOS/Linux):**
```bash
curl -sSL https://raw.githubusercontent.com/nebari-dev/skillctl/main/install.sh | bash
```

**Go:**
```bash
go install github.com/nebari-dev/skillctl/cli@latest
```

## First-time setup

If the user hasn't configured skillctl yet, run the interactive setup:

```bash
skillctl config init
```

This prompts for:
- **API URL** - the registry server (the user's org provides this)
- **Skills directory** - where skills are installed (defaults to `~/.claude/skills`)

For non-interactive setup:

```bash
skillctl config set api_url https://skillctl.example.com
```

## Authentication

For registries that require authentication:

```bash
skillctl auth login
```

This opens a browser-based OIDC device flow. The user visits a URL, enters a code, and the CLI caches the token locally. No manual token management needed.

Check auth status:

```bash
skillctl auth status
```

If a command fails with "Not authenticated", prompt the user to run `skillctl auth login`.

## Discovering skills

Browse available skills:

```bash
# List all skills
skillctl explore

# Filter by tag
skillctl explore --tag go --tag testing

# Filter by source (internal or external)
skillctl explore --source internal
```

Show details for a specific skill:

```bash
skillctl explore show <name>

# Include the full skill content
skillctl explore show <name> --verbose
```

## Installing skills

Install a skill (latest version):

```bash
skillctl install <name>
```

Install a specific version:

```bash
skillctl install <name>@<version>
```

Install with digest verification:

```bash
skillctl install <name>@<version> --digest sha256:<hash>
```

Skills are installed to the configured skills directory (default `~/.claude/skills/<name>.md`). After installing, the skill is immediately available in Claude Code sessions.

## Publishing skills

Publish a skill from a local file:

```bash
skillctl publish \
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
skillctl explore --tag X
skillctl explore show <interesting-skill> --verbose
skillctl install <interesting-skill>
```

### "I want to share a skill I wrote"

```bash
# Make sure you're authenticated
skillctl auth login

# Publish it
skillctl publish --name my-skill --version 1.0.0 --description "Does X" --file ./skill.md --tag relevant-tag
```

### "I want to update an installed skill"

```bash
# Check what's available
skillctl explore show <name>

# Install the new version (overwrites the old one)
skillctl install <name>@<new-version>
```

### "A command says I'm not authenticated"

```bash
skillctl auth status
# If expired or not logged in:
skillctl auth login
```

## Troubleshooting

- **"Cannot reach server"** - check `skillctl config get api_url` and verify the server is reachable
- **"Not authenticated"** - run `skillctl auth login`
- **"Server does not require authentication"** - the registry is running in dev mode, no login needed
- **"Version X already exists"** - versions are immutable, bump the version number
- **"Permission denied"** - you're not the owner of this skill (only the original publisher can update it)
