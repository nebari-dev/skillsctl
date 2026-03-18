# CLI Commands Design Spec

## Goal

Make skillsctl usable end-to-end by adding config management, publish, and install commands to the CLI.

## Commands

### config init

Interactive first-time setup. Prompts for configurable values and writes `~/.config/skillsctl/config.yaml`.

```
$ skillsctl config init
No configuration found. Let's set up skillsctl.

API URL [http://localhost:8080]: https://skillsctl.myorg.com
Skills directory [/home/user/.claude/skills]:

Config saved to /home/user/.config/skillsctl/config.yaml
```

Behavior:
- If config file already exists, print "Config already exists at <path>. Use --force to overwrite." and exit.
- `--force` flag overwrites existing config.
- Empty input accepts the default (shown in brackets).
- Defaults shown in brackets are expanded absolute paths (no tilde).
- Creates parent directories if needed.
- Reads prompts from stdin, writes prompts to stderr so stdout stays clean.
- Stores expanded absolute paths in the config file for portability.

Output file (`~/.config/skillsctl/config.yaml`):
```yaml
api_url: https://skillsctl.myorg.com
skills_dir: /home/user/.claude/skills
```

### config set

```
$ skillsctl config set api_url https://skillsctl.myorg.com
```

Behavior:
- Reads and parses the existing YAML config file directly (not through Viper's merged view, to avoid baking in env vars or defaults).
- If no config file exists, creates a new one with just the specified key.
- Sets the key to the given value.
- Writes the file back.
- Valid keys: `api_url`, `skills_dir`. Unknown keys are rejected with an error listing valid keys.
- Creates parent directories if needed.

Implementation note: Do NOT use `viper.WriteConfig()` for writes. Viper serializes its full in-memory state including env vars and defaults, which would bake transient values into the file. Use `go.yaml.in/yaml/v3` or `os.ReadFile`/`os.WriteFile` with a simple map to read-modify-write only the YAML file contents.

### config get

```
$ skillsctl config get api_url
https://skillsctl.myorg.com
```

Behavior:
- Prints the effective value (flag > env > config file > default) followed by a newline.
- Uses Viper to resolve the effective value (this is read-only, so the merged view is correct).
- Unknown keys print an error listing valid keys.
- Output is a single line with a trailing newline, which is standard for CLI tools and works correctly with shell command substitution (`$(...)` strips the trailing newline).

### config list

```
$ skillsctl config list
api_url: https://skillsctl.myorg.com
skills_dir: /home/user/.claude/skills
```

Behavior:
- Prints all known config keys and their effective values, one per line.
- Uses `key: value` format.
- Always prints both `api_url` and `skills_dir`, using defaults if unset.
- `skills_dir` defaults to `$HOME/.claude/skills` (expanded absolute path).

### publish

```
$ skillsctl publish --name my-skill --version 1.0.0 --description "A useful skill" --file ./skill.md --tag go --tag testing
Published my-skill@1.0.0 (sha256:abc123...)
```

Behavior:
- Reads file content from `--file` path.
- Required flags: `--name`, `--version`, `--description`, `--file`. Enforced via Cobra's `MarkFlagRequired`.
- Optional flags: `--tag` (repeatable string slice), `--changelog` (string).
- Calls `PublishSkill` RPC with the content bytes and metadata.
- On success, prints skill name, version, and digest. If server returns empty digest (should not happen), omit the parenthetical.
- On error, maps Connect error codes to user-friendly messages:
  - `AlreadyExists`: "Version 1.0.0 of my-skill already exists"
  - `InvalidArgument`: prints the server's validation message directly
  - `Unauthenticated`: "Not authenticated. Run 'skillsctl auth login' first."
  - `PermissionDenied`: "Permission denied. You are not the owner of this skill."
  - Other: "Error: <server message>"

Flag validation (client-side, before RPC):
- `--file` must exist and be readable.
- File size must be <= 1MB (matches server limit, avoids uploading then failing).

### install

```
$ skillsctl install my-skill
Installed my-skill@1.0.0 to /home/user/.claude/skills/my-skill.md

$ skillsctl install my-skill@0.9.0
Installed my-skill@0.9.0 to /home/user/.claude/skills/my-skill.md
```

Behavior:
- Parses `name[@version]` from the positional argument. If no `@version`, version is empty (server resolves to latest).
- Calls `GetSkillContent` RPC. The `--digest` flag value (if provided) is sent via `GetSkillContentRequest.digest` so the server performs the verification. The client does not compare digests locally.
- Writes content atomically: write to a temp file in the same directory, then `os.Rename` to the final path. This prevents corrupt files if the write is interrupted.
- Creates the skills directory if it doesn't exist.
- Overwrites existing file if present (installing a new version replaces the old one).
- On success, prints the installed version and destination path.
- On error:
  - `NotFound`: "Skill 'my-skill' not found" or "Version 0.9.0 of 'my-skill' not found"
  - `FailedPrecondition` (digest mismatch): "Digest mismatch for my-skill@1.0.0. Content may have been tampered with."
  - Other: "Error: <server message>"

Optional `--digest` flag for verified installs: `skillsctl install my-skill@1.0.0 --digest sha256:abc123`.

## Files to Create/Modify

### New files:
- `cli/cmd/config.go` - config init/set/get/list commands
- `cli/cmd/config_test.go` - tests
- `cli/cmd/publish.go` - publish command
- `cli/cmd/publish_test.go` - tests
- `cli/cmd/install.go` - install command
- `cli/cmd/install_test.go` - tests

### Modified files:
- `cli/cmd/root.go` - register new commands, add `skills_dir` default to Viper
- `cli/internal/api/client.go` - add PublishSkill method
- `cli/internal/testutil/stub_server.go` - add PublishSkill stub with configurable error field

## API Client Additions

```go
// In cli/internal/api/client.go

func (c *Client) PublishSkill(ctx context.Context, name, version, description, changelog string, tags []string, content []byte) (*skillsctlv1.Skill, *skillsctlv1.SkillVersion, error) {
    resp, err := c.registry.PublishSkill(ctx, connect.NewRequest(&skillsctlv1.PublishSkillRequest{
        Name:        name,
        Version:     version,
        Description: description,
        Tags:        tags,
        Changelog:   changelog,
        Content:     content,
    }))
    if err != nil {
        return nil, nil, err
    }
    return resp.Msg.Skill, resp.Msg.Version, nil
}
```

GetSkillContent already exists in the client.

## Config File Handling

Config writes (`config set`, `config init`) read and write the YAML file directly using `go.yaml.in/yaml/v3`, NOT through Viper. This avoids baking env vars and defaults into the file.

Config reads (`config get`, `config list`, and all other commands) use Viper's merged view, which correctly applies the precedence chain.

Config path: `~/.config/skillsctl/config.yaml`.

Precedence (already implemented, unchanged):
1. CLI flags (`--api-url`)
2. Environment variables (`SKILLCTL_API_URL`)
3. Config file
4. Defaults (`api_url`: http://localhost:8080, `skills_dir`: $HOME/.claude/skills)

## Stub Server for Tests

Extend `StubRegistryService` with a `PublishSkill` method and a configurable error field:

```go
type StubRegistryService struct {
    skillsctlv1connect.UnimplementedRegistryServiceHandler
    Skills      []*skillsctlv1.Skill
    Content     map[string][]byte
    PublishErr  error // if set, PublishSkill returns this as a connect.Error
}

func (s *StubRegistryService) PublishSkill(_ context.Context, req *connect.Request[skillsctlv1.PublishSkillRequest]) (*connect.Response[skillsctlv1.PublishSkillResponse], error) {
    if s.PublishErr != nil {
        return nil, s.PublishErr
    }
    skill := &skillsctlv1.Skill{
        Name:          req.Msg.Name,
        LatestVersion: req.Msg.Version,
    }
    ver := &skillsctlv1.SkillVersion{
        Version: req.Msg.Version,
        Digest:  "sha256:stubdigest",
    }
    return connect.NewResponse(&skillsctlv1.PublishSkillResponse{
        Skill:   skill,
        Version: ver,
    }), nil
}
```

This allows tests to inject specific Connect errors (e.g., `connect.NewError(connect.CodeAlreadyExists, ...)`) for failure-path testing.

## Testing Strategy

All commands tested with the existing stub server pattern from `cli/internal/testutil/`. Config commands use `t.TempDir()` for the config file path, overriding Viper's config path in tests.

- `config init`: test interactive prompts (pipe stdin), test --force, test existing config rejection
- `config set/get/list`: test round-trip, test unknown key rejection, test defaults when no file exists
- `publish`: test success output, test file-not-found, test server error mapping (AlreadyExists, Unauthenticated, PermissionDenied)
- `install`: test success with file written to temp dir, test version parsing (name vs name@version), test --digest flag forwarded to server, test atomic write (file exists after success)
- Integration: publish then install in sequence using stub server

## Non-goals

- OIDC auth login (separate chunk)
- Skill manifest file (post-MVP)
- OCI storage (post-MVP)
