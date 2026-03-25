# skillsctl - Skill Registry CLI

You have access to `skillsctl`, a CLI tool for discovering, installing, and publishing Claude Code skills from a shared registry.

## Finding skillsctl

Before using any commands, locate the skillsctl binary:

1. Check if it's on the system PATH: `which skillsctl`
2. If not found, check for a local build in the current directory (e.g. `./skillsctl`, `./bin/skillsctl`) or in the skillsctl repo
3. If neither exists, ask the user if they have a local build before suggesting installation

If skillsctl is not installed anywhere, recommend the method that fits their setup:

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

Before running commands, verify the API URL is configured:

```bash
skillsctl config get api_url
```

If no API URL is set (empty or errors), ask the user for their registry URL and set it:

```bash
skillsctl config set api_url https://skillsctl.example.com
```

Alternatively, run the interactive setup which prompts for all settings:

```bash
skillsctl config init
```

This prompts for:
- **API URL** - the registry server (the user's org provides this)
- **Skills directory** - where skills are installed (defaults to `~/.claude/skills`)

## Authentication

For registries that require authentication:

1. Check auth status first: `skillsctl auth status`
2. If expired or not logged in, ask the user: "Want me to run the login for you? I'll open the browser so you can authenticate."
3. If they agree, run `skillsctl auth login`, extract the URL from the output, and open it in the browser:
   ```bash
   xdg-open <url>   # Linux
   open <url>        # macOS
   ```
4. Tell the user the browser should be open and to complete authentication there.

The login uses an OIDC device flow - the CLI outputs a URL with a code, the user authorizes in the browser, and the CLI caches the token locally. No manual token management needed.

If a command fails with "Not authenticated", follow the same flow above.

## Resolving skill names

Users often refer to skills informally ("the terraform thing", "my go testing skill"). When the user mentions a skill by description rather than exact name:

1. Search installed skills (`ls ~/.claude/skills/`) and the registry (`skillsctl explore`)
2. Match their description to actual skill names
3. **Always confirm before acting:** "Did you mean `review-iac`?"
4. If ambiguous, list candidates and ask

This applies to install, publish, explore show, and any command that takes a skill name.

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

Skills are installed to the configured skills directory (default `~/.claude/skills/<name>/SKILL.md`). After installing, the skill is immediately available in Claude Code sessions.

## Publishing skills

Authentication is required before publishing. Run `skillsctl auth login` first if you haven't already.

### Finding the skill file

A skill is a single markdown file. Common locations to look:
- **Project skills directory:** `skills/<name>.md` in the current repo
- **Installed skills:** `~/.claude/skills/<name>/SKILL.md`
- **Plugin cache:** `~/.claude/plugins/cache/` (for skills from plugin packages)

To publish an already-installed skill, use the installed path:
```bash
skillsctl publish --name review-iac --version 1.0.0 \
  --description "Review IaC pull requests" \
  --file ~/.claude/skills/review-iac/SKILL.md
```

### Publish command

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

### "I want to publish all my installed skills"

1. List installed skills: `ls ~/.claude/skills/`
2. Check what's already published: `skillsctl explore`
3. For each installed skill not yet in the registry:
   - Read the skill's SKILL.md frontmatter to extract name and description
   - Generate reasonable tags from the skill's description and content
   - Publish with version `1.0.0` (or the next version if already published)
   - Use the installed path: `~/.claude/skills/<name>/SKILL.md`
4. Show a summary table of what was published

For skills that are already in the registry, compare versions and ask if the user wants to publish an update.

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
