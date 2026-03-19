---
title: "Configuration"
weight: 60
---

# Configuration

skillsctl reads configuration from a YAML file at `~/.config/skillsctl/config.yaml`. You can also override any setting with environment variables or command-line flags.

## Initialize configuration

Run the interactive setup to create the config file:

```bash
skillsctl config init
```

```
No configuration found. Let's set up skillsctl.

API URL [http://localhost:8080]: https://skills.corp.example.com
Skills directory [/home/you/.claude/skills]:

Config saved to /home/you/.config/skillsctl/config.yaml
```

This writes a config file with the values you entered. Run it again any time to change them.

## View current configuration

List all current configuration values:

```bash
skillsctl config list
```

```
api_url: https://skills.corp.example.com
```

## Get a single value

```bash
skillsctl config get api_url
```

```
https://skills.corp.example.com
```

## Set a value

```bash
skillsctl config set api_url https://skills-staging.corp.example.com
```

```
Updated api_url in /home/you/.config/skillsctl/config.yaml
```

## Configuration options

| Key | Description | Default |
|-----|-------------|---------|
| `api_url` | Registry server URL | `http://localhost:8080` |
| `skills_dir` | Directory where skills are installed | `~/.claude/skills` |

## Environment variables

Every configuration key has a corresponding environment variable with the `SKILLCTL_` prefix and the key uppercased:

| Environment variable | Config key | Example |
|---------------------|------------|---------|
| `SKILLCTL_API_URL` | `api_url` | `https://skills.corp.example.com` |

Environment variables are useful in CI/CD pipelines and containerized environments where you don't want a config file on disk:

```bash
SKILLCTL_API_URL=https://skills.corp.example.com skillsctl explore
```

## Precedence

When the same setting is provided in multiple places, skillsctl uses this order (highest to lowest priority):

1. **Flag** - e.g. `--api-url https://...` on the command line
2. **Environment variable** - e.g. `SKILLCTL_API_URL`
3. **Config file** - `~/.config/skillsctl/config.yaml`
4. **Default** - built-in default value

This means a flag on a single command always wins, making it easy to override the config for one-off operations without editing anything.

## Config file format

The config file is standard YAML:

```yaml
api_url: https://skills.corp.example.com
```

You can edit it directly with any text editor. skillsctl writes config values with go.yaml.in/yaml/v3 directly (not Viper) so environment variables are never baked into the file.

## Credentials file

Authentication tokens are stored separately at `~/.config/skillsctl/credentials.json`. This file is managed automatically by `skillsctl auth login` and `skillsctl auth logout`. Do not edit it manually.

## Next steps

- [Publishing skills]({{< relref "/getting-started/publishing" >}}) - share skills with your team
