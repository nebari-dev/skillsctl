---
title: "Installation"
weight: 10
---

# Installation

SkillsCtl is distributed as a single static binary. Choose the method that fits your environment.

## Homebrew (recommended for macOS and Linux)

Homebrew is the recommended install method for macOS and Linux users. It handles upgrades cleanly and integrates with your system PATH.

```bash
brew tap nebari-dev/tap
brew install skillsctl
```

To upgrade later:

```bash
brew upgrade skillsctl
```

## curl installer

For Linux and macOS systems without Homebrew, use the install script to download the latest release:

```bash
curl -fsSL https://raw.githubusercontent.com/nebari-dev/skillsctl/main/install.sh | bash
```

The script installs the binary to `/usr/local/bin/skillsctl` by default. Set `INSTALL_DIR` to override:

```bash
curl -fsSL https://raw.githubusercontent.com/nebari-dev/skillsctl/main/install.sh | INSTALL_DIR=$HOME/.local/bin bash
```

## go install

If you have Go 1.21 or later installed, you can install directly from source:

```bash
go install github.com/nebari-dev/skillsctl/cli@latest
```

The binary is placed in `$(go env GOPATH)/bin/skillsctl`. Make sure that directory is in your `PATH`.

## Build from source

Clone the repository and build manually:

```bash
git clone https://github.com/nebari-dev/skillsctl.git
cd skillsctl
CGO_ENABLED=0 go build -o skillsctl ./cli
```

Move the binary somewhere on your `PATH`:

```bash
mv skillsctl /usr/local/bin/
```

## Verify the installation

After installing, confirm the binary is reachable and check the version:

```bash
skillsctl --version
```

Expected output:

```
skillsctl version 0.1.0
```

## Shell completion

skillsctl supports shell completion for bash, zsh, fish, and PowerShell. Generate and install the completion script for your shell:

```bash
# bash
skillsctl completion bash > /etc/bash_completion.d/skillsctl

# zsh (add to ~/.zshrc or a file sourced by it)
skillsctl completion zsh > "${fpath[1]}/_skillsctl"

# fish
skillsctl completion fish > ~/.config/fish/completions/skillsctl.fish
```

## Next steps

- [Quick start]({{< relref "/getting-started/quickstart" >}}) - run a local registry server
- [Configuration]({{< relref "/getting-started/configuration" >}}) - point the CLI at your registry
