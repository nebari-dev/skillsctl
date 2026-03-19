---
title: "CLI Reference"
weight: 20
bookCollapseSection: true
---

# CLI reference

`skillsctl` is the command-line interface for discovering, installing, and publishing Claude Code skills.

## Commands

| Command | Description |
|---------|-------------|
| [explore]({{< relref "/cli/explore" >}}) | Browse skills in the registry |
| [install]({{< relref "/cli/install" >}}) | Install a skill to your local skills directory |
| [publish]({{< relref "/cli/publish" >}}) | Publish a skill to the registry |
| [auth]({{< relref "/cli/auth" >}}) | Log in, check status, or log out |
| [config]({{< relref "/cli/config" >}}) | Manage CLI configuration |

## Global flags

These flags apply to every command.

| Flag | Description |
|------|-------------|
| `--api-url URL` | Registry server URL. Overrides `api_url` from config. |
| `--credentials-path PATH` | Path to credentials file. Default: `~/.config/skillsctl/credentials.json`. |
| `--version` | Print the SkillsCtl version and exit. |
| `--help` | Print help for the current command. |

## Configuration

The CLI reads `~/.config/skillsctl/config.yaml` for default values. Use `skillsctl config init` to create this file interactively, or set individual keys with `skillsctl config set`. Global flags override config file values, which override built-in defaults.

See the [config command reference]({{< relref "/cli/config" >}}) for details.
