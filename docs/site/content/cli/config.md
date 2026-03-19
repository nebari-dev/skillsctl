---
title: "config"
weight: 50
---

# config

Manage the CLI configuration file at `~/.config/skillsctl/config.yaml`.

## Synopsis

```
SkillsCtl config init [--force]
SkillsCtl config set KEY VALUE
SkillsCtl config get KEY
SkillsCtl config list
```

## Subcommands

| Subcommand | Description |
|------------|-------------|
| `init` | Create the config file interactively |
| `set KEY VALUE` | Set a single configuration key |
| `get KEY` | Print the effective value for a key |
| `list` | Print all configuration values |

## Flags

| Flag | Description |
|------|-------------|
| `--config-path PATH` | Path to config file. Default: `~/.config/skillsctl/config.yaml`. |
| `--force` | (init only) Overwrite an existing config file. |

## Valid keys

| Key | Description | Default |
|-----|-------------|---------|
| `api_url` | Registry server URL | `http://localhost:8080` |
| `skills_dir` | Directory where installed skills are written | `~/.claude/skills` |

## Examples

Run interactive setup:

```bash
SkillsCtl config init
```

```
No configuration found. Let's set up SkillsCtl.

API URL [http://localhost:8080]: https://skills.example.com
Skills directory [/home/you/.claude/skills]:

Config saved to /home/you/.config/skillsctl/config.yaml
```

Overwrite an existing config:

```bash
SkillsCtl config init --force
```

Set a single key:

```bash
SkillsCtl config set api_url https://skills.example.com
```

Get the effective value for a key (config file value, unless overridden by a global flag):

```bash
SkillsCtl config get api_url
```

```
https://skills.example.com
```

List all configured values:

```bash
SkillsCtl config list
```

```
api_url: https://skills.example.com
skills_dir: /home/you/.claude/skills
```

## Common errors

**Error: config file already exists. Use --force to overwrite.**
Run `skillsctl config init --force` to replace the existing file.

**Error: unknown key "KEY"**
The key is not recognized. Valid keys are `api_url` and `skills_dir`.

**Error: config file not found**
No config file exists at the expected path. Run `skillsctl config init` to create one, or pass `--api-url` as a global flag.
