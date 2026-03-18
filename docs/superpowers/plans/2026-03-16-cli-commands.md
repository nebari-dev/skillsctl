# CLI Commands Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add config management, publish, and install commands to make the CLI usable end-to-end.

**Architecture:** Six new commands (`config init/set/get/list`, `publish`, `install`) added as Cobra subcommands. Config writes use `gopkg.in/yaml.v3` directly (not Viper) to avoid baking env vars into the file. Config reads use Viper's merged view. Publish and install commands use the existing API client + stub server test pattern.

**Tech Stack:** Cobra, Viper, gopkg.in/yaml.v3, ConnectRPC client

**Spec:** `docs/superpowers/specs/2026-03-16-cli-commands-design.md`

---

## File Structure

### New files:
- `cli/cmd/config.go` - config command group with init/set/get/list subcommands
- `cli/cmd/config_test.go` - tests for all config commands
- `cli/cmd/publish.go` - publish command
- `cli/cmd/publish_test.go` - tests for publish
- `cli/cmd/install.go` - install command
- `cli/cmd/install_test.go` - tests for install

### Modified files:
- `cli/cmd/root.go` - register config/publish/install commands, add `skills_dir` default
- `cli/internal/api/client.go` - add PublishSkill method, add digest param to GetSkillContent
- `cli/internal/testutil/stub_server.go` - add PublishSkill stub with configurable error

---

## Chunk 1: Stub Server and API Client Updates

### Task 1: Extend stub server with PublishSkill and update GetSkillContent

**Files:**
- Modify: `cli/internal/testutil/stub_server.go`
- Modify: `cli/internal/api/client.go`
- Modify: `cli/internal/api/client_test.go`

- [ ] **Step 1: Add PublishSkill to stub server and PublishErr field**

In `cli/internal/testutil/stub_server.go`, add the `PublishErr` field and `PublishSkill` method:

```go
type StubRegistryService struct {
	skillsctlv1connect.UnimplementedRegistryServiceHandler
	Skills     []*skillsctlv1.Skill
	Content    map[string][]byte
	PublishErr error // if set, PublishSkill returns this error
}

func (s *StubRegistryService) PublishSkill(_ context.Context, req *connect.Request[skillsctlv1.PublishSkillRequest]) (*connect.Response[skillsctlv1.PublishSkillResponse], error) {
	if s.PublishErr != nil {
		return nil, s.PublishErr
	}
	skill := &skillsctlv1.Skill{
		Name:          req.Msg.Name,
		LatestVersion: req.Msg.Version,
		Owner:         "dev-user",
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

Also update `GetSkillContent` to forward the `Digest` field from the request for verification:

```go
func (s *StubRegistryService) GetSkillContent(_ context.Context, req *connect.Request[skillsctlv1.GetSkillContentRequest]) (*connect.Response[skillsctlv1.GetSkillContentResponse], error) {
	if s.Content == nil {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	content, ok := s.Content[req.Msg.Name]
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	if req.Msg.Digest != "" && req.Msg.Digest != "sha256:gooddigest" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("digest mismatch"))
	}
	return connect.NewResponse(&skillsctlv1.GetSkillContentResponse{
		Content: content,
		Version: &skillsctlv1.SkillVersion{Version: "1.0.0", Digest: "sha256:gooddigest"},
	}), nil
}
```

Add `"fmt"` to the imports.

Also add a constructor that accepts all fields:

```go
// NewStubServerFull starts a test server with all configurable fields.
func NewStubServerFull(t *testing.T, skills []*skillsctlv1.Skill, content map[string][]byte, publishErr error) *httptest.Server {
	t.Helper()
	stub := &StubRegistryService{Skills: skills, Content: content, PublishErr: publishErr}
	mux := http.NewServeMux()
	path, handler := skillsctlv1connect.NewRegistryServiceHandler(stub)
	mux.Handle(path, handler)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}
```

- [ ] **Step 2: Add PublishSkill method and update GetSkillContent signature in API client**

In `cli/internal/api/client.go`, add `PublishSkill` and update `GetSkillContent` to accept a digest:

```go
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

Update existing `GetSkillContent` to accept digest:

```go
func (c *Client) GetSkillContent(ctx context.Context, name, version, digest string) ([]byte, *skillsctlv1.SkillVersion, error) {
	resp, err := c.registry.GetSkillContent(ctx, connect.NewRequest(&skillsctlv1.GetSkillContentRequest{
		Name:    name,
		Version: version,
		Digest:  digest,
	}))
	if err != nil {
		return nil, nil, err
	}
	return resp.Msg.Content, resp.Msg.Version, nil
}
```

- [ ] **Step 3: Fix caller of GetSkillContent in explore.go**

In `cli/cmd/explore.go`, update the `GetSkillContent` call to pass empty digest:

```go
content, _, err := client.GetSkillContent(cmd.Context(), args[0], "", "")
```

- [ ] **Step 4: Add API client test for PublishSkill**

Add to `cli/internal/api/client_test.go`:

```go
func TestClient_PublishSkill(t *testing.T) {
	ts := testutil.NewStubServer(t, nil)
	client := api.NewClient(ts.URL)

	skill, ver, err := client.PublishSkill(context.Background(),
		"test-skill", "1.0.0", "A test", "Initial", []string{"go"}, []byte("content"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skill.Name != "test-skill" {
		t.Errorf("expected name test-skill, got %q", skill.Name)
	}
	if ver.Digest == "" {
		t.Error("expected non-empty digest")
	}
}
```

- [ ] **Step 5: Run tests and verify**

Run: `go test ./cli/... -race -count=1`

Expected: All PASS.

- [ ] **Step 6: Commit**

```bash
git add cli/internal/testutil/stub_server.go cli/internal/api/client.go cli/internal/api/client_test.go cli/cmd/explore.go
git commit -m "feat: add PublishSkill to API client and extend stub server"
```

---

## Chunk 2: Config Commands

### Task 2: Implement config init command

**Files:**
- Create: `cli/cmd/config.go`
- Create: `cli/cmd/config_test.go`
- Modify: `cli/cmd/root.go`

- [ ] **Step 1: Write failing test for config init**

Create `cli/cmd/config_test.go`:

```go
package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nebari-dev/skillsctl/cli/cmd"
)

func TestConfigInit(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	var buf bytes.Buffer
	root := cmd.NewRootCmd()
	root.SetOut(&buf)
	root.SetErr(&buf)
	// Pipe stdin with defaults (just press enter twice)
	root.SetIn(strings.NewReader("\n\n"))
	root.SetArgs([]string{"config", "init", "--config-path", configPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify config file was created
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "api_url") {
		t.Errorf("expected api_url in config, got:\n%s", content)
	}
	if !strings.Contains(content, "skills_dir") {
		t.Errorf("expected skills_dir in config, got:\n%s", content)
	}
}

func TestConfigInit_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configPath, []byte("api_url: http://example.com\n"), 0644)

	var buf bytes.Buffer
	root := cmd.NewRootCmd()
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"config", "init", "--config-path", configPath})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for existing config")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestConfigInit_Force(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configPath, []byte("api_url: http://old.com\n"), 0644)

	var buf bytes.Buffer
	root := cmd.NewRootCmd()
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetIn(strings.NewReader("http://new.com\n\n"))
	root.SetArgs([]string{"config", "init", "--config-path", configPath, "--force"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	if !strings.Contains(string(data), "http://new.com") {
		t.Errorf("expected new URL in config, got:\n%s", string(data))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cli/cmd/... -run TestConfig -v -race -count=1`

Expected: FAIL - command not registered.

- [ ] **Step 3: Implement config command group and init subcommand**

Create `cli/cmd/config.go`:

```go
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

// configPath is overridable via --config-path for testing.
var configPath string

// validConfigKeys lists the keys that config set/get accept.
var validConfigKeys = []string{"api_url", "skills_dir"}

func addConfigCmd(root *cobra.Command) {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage skillsctl configuration",
	}

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Interactive first-time setup",
		RunE:  runConfigInit,
	}
	initCmd.Flags().Bool("force", false, "Overwrite existing config")

	setCmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(2),
		RunE:  runConfigSet,
	}

	getCmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get a config value",
		Args:  cobra.ExactArgs(1),
		RunE:  runConfigGet,
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all config values",
		RunE:  runConfigList,
	}

	configCmd.PersistentFlags().StringVar(&configPath, "config-path", "", "Config file path (for testing)")
	configCmd.AddCommand(initCmd, setCmd, getCmd, listCmd)
	root.AddCommand(configCmd)
}

func resolveConfigPath() string {
	if configPath != "" {
		return configPath
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "skillsctl", "config.yaml")
}

func runConfigInit(cmd *cobra.Command, _ []string) error {
	path := resolveConfigPath()
	force, _ := cmd.Flags().GetBool("force")

	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config already exists at %s. Use --force to overwrite", path)
		}
	}

	home, _ := os.UserHomeDir()
	defaultAPI := "http://localhost:8080"
	defaultSkills := filepath.Join(home, ".claude", "skills")

	reader := bufio.NewReader(cmd.InOrStdin())

	fmt.Fprintf(cmd.ErrOrStderr(), "No configuration found. Let's set up skillsctl.\n\n")
	fmt.Fprintf(cmd.ErrOrStderr(), "API URL [%s]: ", defaultAPI)
	apiInput, _ := reader.ReadString('\n')
	apiInput = strings.TrimSpace(apiInput)
	if apiInput == "" {
		apiInput = defaultAPI
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Skills directory [%s]: ", defaultSkills)
	skillsInput, _ := reader.ReadString('\n')
	skillsInput = strings.TrimSpace(skillsInput)
	if skillsInput == "" {
		skillsInput = defaultSkills
	}

	cfg := map[string]string{
		"api_url":    apiInput,
		"skills_dir": skillsInput,
	}

	if err := writeConfigFile(path, cfg); err != nil {
		return err
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "\nConfig saved to %s\n", path)
	return nil
}

// writeConfigFile writes a map to a YAML file, creating parent dirs as needed.
func writeConfigFile(path string, data map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	out, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, out, 0644)
}

// readConfigFile reads the YAML config file into a map. Returns empty map if file doesn't exist.
func readConfigFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return make(map[string]string), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := make(map[string]string)
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

func isValidKey(key string) bool {
	for _, k := range validConfigKeys {
		if k == key {
			return true
		}
	}
	return false
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key, value := args[0], args[1]
	if !isValidKey(key) {
		return fmt.Errorf("unknown config key %q. Valid keys: %s", key, strings.Join(validConfigKeys, ", "))
	}

	path := resolveConfigPath()
	cfg, err := readConfigFile(path)
	if err != nil {
		return err
	}
	cfg[key] = value
	return writeConfigFile(path, cfg)
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	key := args[0]
	if !isValidKey(key) {
		return fmt.Errorf("unknown config key %q. Valid keys: %s", key, strings.Join(validConfigKeys, ", "))
	}

	loadConfigOverride()
	fmt.Fprintln(cmd.OutOrStdout(), viper.GetString(key))
	return nil
}

func runConfigList(cmd *cobra.Command, _ []string) error {
	loadConfigOverride()
	for _, key := range validConfigKeys {
		fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", key, viper.GetString(key))
	}
	return nil
}

// loadConfigOverride re-reads Viper config from the --config-path if set.
// This ensures config get/list respect the override path in tests.
func loadConfigOverride() {
	if configPath != "" {
		viper.SetConfigFile(configPath)
		_ = viper.ReadInConfig()
	}
}
```

- [ ] **Step 4: Register config command in root.go**

In `cli/cmd/root.go`, add after `addExploreCmd(rootCmd)`:

```go
	addConfigCmd(rootCmd)
```

Also add the `skills_dir` default that was missing from root.go's Viper setup:

```go
	cobra.OnInitialize(func() {
		home, _ := os.UserHomeDir()
		viper.SetDefault("api_url", "http://localhost:8080")
		viper.SetDefault("skills_dir", filepath.Join(home, ".claude", "skills"))
		// ... rest unchanged
	})
```

Add `"path/filepath"` to root.go imports.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./cli/cmd/... -run TestConfig -v -race -count=1`

Expected: All PASS.

- [ ] **Step 6: Commit**

```bash
git add cli/cmd/config.go cli/cmd/config_test.go cli/cmd/root.go
git commit -m "feat: add config init/set/get/list commands"
```

### Task 3: Add config set/get/list tests

**Files:**
- Modify: `cli/cmd/config_test.go`

- [ ] **Step 1: Write tests for set/get/list**

Add to `cli/cmd/config_test.go`:

```go
func TestConfigSetAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Set a value
	root := cmd.NewRootCmd()
	root.SetArgs([]string{"config", "set", "api_url", "http://custom.com", "--config-path", configPath})
	if err := root.Execute(); err != nil {
		t.Fatalf("set: %v", err)
	}

	// Read the file directly to verify
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "http://custom.com") {
		t.Errorf("expected custom URL in file, got:\n%s", string(data))
	}
}

func TestConfigSet_UnknownKey(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"config", "set", "unknown_key", "value", "--config-path", configPath})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
	if !strings.Contains(err.Error(), "unknown config key") {
		t.Errorf("expected 'unknown config key' error, got: %v", err)
	}
}

func TestConfigGet_UnknownKey(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"config", "get", "bogus", "--config-path", configPath})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
}

func TestConfigList(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	var buf bytes.Buffer
	root := cmd.NewRootCmd()
	root.SetOut(&buf)
	root.SetArgs([]string{"config", "list", "--config-path", configPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "api_url:") {
		t.Errorf("expected api_url in list, got:\n%s", output)
	}
	if !strings.Contains(output, "skills_dir:") {
		t.Errorf("expected skills_dir in list, got:\n%s", output)
	}
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./cli/cmd/... -run TestConfig -v -race -count=1`

Expected: All PASS.

- [ ] **Step 3: Run all CLI tests**

Run: `go test ./cli/... -race -count=1`

Expected: All PASS.

- [ ] **Step 4: Commit**

```bash
git add cli/cmd/config_test.go
git commit -m "test: add config set/get/list tests"
```

---

## Chunk 3: Publish Command

### Task 4: Implement publish command

**Files:**
- Create: `cli/cmd/publish.go`
- Create: `cli/cmd/publish_test.go`
- Modify: `cli/cmd/root.go`

- [ ] **Step 1: Write failing tests for publish**

Create `cli/cmd/publish_test.go`:

```go
package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"github.com/nebari-dev/skillsctl/cli/cmd"
	"github.com/nebari-dev/skillsctl/cli/internal/testutil"
)

func TestPublish(t *testing.T) {
	ts := testutil.NewStubServer(t, nil)

	// Create a temp skill file
	tmpDir := t.TempDir()
	skillFile := filepath.Join(tmpDir, "my-skill.md")
	os.WriteFile(skillFile, []byte("# My Skill\nDoes stuff"), 0644)

	var buf bytes.Buffer
	root := cmd.NewRootCmd()
	root.SetOut(&buf)
	root.SetArgs([]string{
		"publish",
		"--name", "my-skill",
		"--version", "1.0.0",
		"--description", "A test skill",
		"--file", skillFile,
		"--tag", "go",
		"--tag", "testing",
		"--api-url", ts.URL,
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Published my-skill@1.0.0") {
		t.Errorf("expected success message, got:\n%s", output)
	}
	if !strings.Contains(output, "sha256:") {
		t.Errorf("expected digest in output, got:\n%s", output)
	}
}

func TestPublish_FileNotFound(t *testing.T) {
	ts := testutil.NewStubServer(t, nil)

	root := cmd.NewRootCmd()
	root.SetArgs([]string{
		"publish",
		"--name", "my-skill",
		"--version", "1.0.0",
		"--description", "desc",
		"--file", "/nonexistent/file.md",
		"--api-url", ts.URL,
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestPublish_FileTooLarge(t *testing.T) {
	ts := testutil.NewStubServer(t, nil)

	tmpDir := t.TempDir()
	bigFile := filepath.Join(tmpDir, "big.md")
	os.WriteFile(bigFile, make([]byte, 1024*1024+1), 0644)

	root := cmd.NewRootCmd()
	root.SetArgs([]string{
		"publish",
		"--name", "my-skill",
		"--version", "1.0.0",
		"--description", "desc",
		"--file", bigFile,
		"--api-url", ts.URL,
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for large file")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("expected size error, got: %v", err)
	}
}

func TestPublish_AlreadyExists(t *testing.T) {
	ts := testutil.NewStubServerFull(t, nil, nil,
		connect.NewError(connect.CodeAlreadyExists, nil))

	tmpDir := t.TempDir()
	skillFile := filepath.Join(tmpDir, "skill.md")
	os.WriteFile(skillFile, []byte("content"), 0644)

	var buf bytes.Buffer
	root := cmd.NewRootCmd()
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{
		"publish",
		"--name", "my-skill",
		"--version", "1.0.0",
		"--description", "desc",
		"--file", skillFile,
		"--api-url", ts.URL,
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' message, got: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cli/cmd/... -run TestPublish -v -race -count=1`

Expected: FAIL - command not registered.

- [ ] **Step 3: Implement publish command**

Create `cli/cmd/publish.go`:

```go
package cmd

import (
	"errors"
	"fmt"
	"os"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	"github.com/nebari-dev/skillsctl/cli/internal/api"
)

const maxContentBytes = 1024 * 1024 // 1MB

func addPublishCmd(root *cobra.Command) {
	var (
		name        string
		version     string
		description string
		filePath    string
		tags        []string
		changelog   string
	)

	publishCmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish a skill to the registry",
		RunE: func(cmd *cobra.Command, _ []string) error {
			content, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("read file %s: %w", filePath, err)
			}
			if len(content) > maxContentBytes {
				return fmt.Errorf("file exceeds maximum size of %d bytes", maxContentBytes)
			}

			client := api.NewClient(getAPIURL())
			_, ver, err := client.PublishSkill(cmd.Context(), name, version, description, changelog, tags, content)
			if err != nil {
				return mapPublishError(err, name, version)
			}

			if ver.Digest != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Published %s@%s (%s)\n", name, version, ver.Digest)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Published %s@%s\n", name, version)
			}
			return nil
		},
	}

	publishCmd.Flags().StringVar(&name, "name", "", "Skill name")
	publishCmd.Flags().StringVar(&version, "version", "", "Skill version (semver)")
	publishCmd.Flags().StringVar(&description, "description", "", "Skill description")
	publishCmd.Flags().StringVar(&filePath, "file", "", "Path to skill content file")
	publishCmd.Flags().StringSliceVar(&tags, "tag", nil, "Tags (repeatable)")
	publishCmd.Flags().StringVar(&changelog, "changelog", "", "Version changelog")

	publishCmd.MarkFlagRequired("name")
	publishCmd.MarkFlagRequired("version")
	publishCmd.MarkFlagRequired("description")
	publishCmd.MarkFlagRequired("file")

	root.AddCommand(publishCmd)
}

func mapPublishError(err error, name, version string) error {
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		return err
	}
	switch connectErr.Code() {
	case connect.CodeAlreadyExists:
		return fmt.Errorf("version %s of %s already exists", version, name)
	case connect.CodeUnauthenticated:
		return fmt.Errorf("not authenticated. Run 'skillsctl auth login' first")
	case connect.CodePermissionDenied:
		return fmt.Errorf("permission denied. You are not the owner of this skill")
	case connect.CodeInvalidArgument:
		return fmt.Errorf("%s", connectErr.Message())
	default:
		return fmt.Errorf("error: %s", connectErr.Message())
	}
}
```

- [ ] **Step 4: Register publish command in root.go**

In `cli/cmd/root.go`, add after `addConfigCmd(rootCmd)`:

```go
	addPublishCmd(rootCmd)
```

- [ ] **Step 5: Run tests**

Run: `go test ./cli/cmd/... -run TestPublish -v -race -count=1`

Expected: All PASS.

- [ ] **Step 6: Run all CLI tests**

Run: `go test ./cli/... -race -count=1`

Expected: All PASS.

- [ ] **Step 7: Commit**

```bash
git add cli/cmd/publish.go cli/cmd/publish_test.go cli/cmd/root.go
git commit -m "feat: add publish command"
```

---

## Chunk 4: Install Command

### Task 5: Implement install command

**Files:**
- Create: `cli/cmd/install.go`
- Create: `cli/cmd/install_test.go`
- Modify: `cli/cmd/root.go`

- [ ] **Step 1: Write failing tests for install**

Create `cli/cmd/install_test.go`:

```go
package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nebari-dev/skillsctl/cli/cmd"
	"github.com/nebari-dev/skillsctl/cli/internal/testutil"
)

func TestInstall(t *testing.T) {
	content := map[string][]byte{
		"my-skill": []byte("# My Skill\nDoes stuff"),
	}
	ts := testutil.NewStubServerWithContent(t, testutil.SeedSkills(), content)

	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")

	var buf bytes.Buffer
	root := cmd.NewRootCmd()
	root.SetOut(&buf)
	root.SetArgs([]string{
		"install", "my-skill",
		"--api-url", ts.URL,
		"--skills-dir", skillsDir,
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Installed my-skill@1.0.0") {
		t.Errorf("expected success message, got:\n%s", output)
	}

	// Verify file was written
	installed, err := os.ReadFile(filepath.Join(skillsDir, "my-skill.md"))
	if err != nil {
		t.Fatalf("skill file not created: %v", err)
	}
	if string(installed) != "# My Skill\nDoes stuff" {
		t.Errorf("unexpected file content: %q", string(installed))
	}
}

func TestInstall_WithVersion(t *testing.T) {
	content := map[string][]byte{
		"my-skill": []byte("content"),
	}
	ts := testutil.NewStubServerWithContent(t, testutil.SeedSkills(), content)

	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")

	var buf bytes.Buffer
	root := cmd.NewRootCmd()
	root.SetOut(&buf)
	root.SetArgs([]string{
		"install", "my-skill@0.9.0",
		"--api-url", ts.URL,
		"--skills-dir", skillsDir,
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File should exist
	if _, err := os.Stat(filepath.Join(skillsDir, "my-skill.md")); err != nil {
		t.Fatalf("skill file not created: %v", err)
	}
}

func TestInstall_NotFound(t *testing.T) {
	ts := testutil.NewStubServer(t, testutil.SeedSkills())

	tmpDir := t.TempDir()

	root := cmd.NewRootCmd()
	root.SetArgs([]string{
		"install", "nonexistent",
		"--api-url", ts.URL,
		"--skills-dir", tmpDir,
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent skill")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestInstall_DigestMismatch(t *testing.T) {
	content := map[string][]byte{
		"my-skill": []byte("content"),
	}
	ts := testutil.NewStubServerWithContent(t, testutil.SeedSkills(), content)

	tmpDir := t.TempDir()

	root := cmd.NewRootCmd()
	root.SetArgs([]string{
		"install", "my-skill@1.0.0",
		"--digest", "sha256:baddigest",
		"--api-url", ts.URL,
		"--skills-dir", tmpDir,
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for digest mismatch")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cli/cmd/... -run TestInstall -v -race -count=1`

Expected: FAIL - command not registered.

- [ ] **Step 3: Implement install command**

Create `cli/cmd/install.go`:

```go
package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/nebari-dev/skillsctl/cli/internal/api"
)

func addInstallCmd(root *cobra.Command) {
	var (
		digest    string
		skillsDir string
	)

	installCmd := &cobra.Command{
		Use:   "install <name[@version]>",
		Short: "Install a skill from the registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, version := parseNameVersion(args[0])

			dir := skillsDir
			if dir == "" {
				dir = viper.GetString("skills_dir")
			}

			client := api.NewClient(getAPIURL())
			content, ver, err := client.GetSkillContent(cmd.Context(), name, version, digest)
			if err != nil {
				return mapInstallError(err, name, version)
			}

			destPath := filepath.Join(dir, name+".md")
			if err := atomicWrite(destPath, content); err != nil {
				return fmt.Errorf("write skill file: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Installed %s@%s to %s\n", name, ver.Version, destPath)
			return nil
		},
	}

	installCmd.Flags().StringVar(&digest, "digest", "", "Expected content digest for verification")
	installCmd.Flags().StringVar(&skillsDir, "skills-dir", "", "Override skills directory")

	root.AddCommand(installCmd)
}

// parseNameVersion splits "name@version" into (name, version).
// If no @ is present, version is empty (server resolves to latest).
func parseNameVersion(arg string) (string, string) {
	if idx := strings.LastIndex(arg, "@"); idx > 0 {
		return arg[:idx], arg[idx+1:]
	}
	return arg, ""
}

// atomicWrite writes data to a temp file then renames it to the destination.
func atomicWrite(destPath string, data []byte) error {
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".skillsctl-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename to %s: %w", destPath, err)
	}
	return nil
}

func mapInstallError(err error, name, version string) error {
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		return err
	}
	switch connectErr.Code() {
	case connect.CodeNotFound:
		if version != "" {
			return fmt.Errorf("version %s of skill %q not found", version, name)
		}
		return fmt.Errorf("skill %q not found", name)
	case connect.CodeFailedPrecondition:
		return fmt.Errorf("digest mismatch for %s@%s. Content may have been tampered with", name, version)
	default:
		return fmt.Errorf("error: %s", connectErr.Message())
	}
}
```

- [ ] **Step 4: Register install command in root.go**

In `cli/cmd/root.go`, add after `addPublishCmd(rootCmd)`:

```go
	addInstallCmd(rootCmd)
```

- [ ] **Step 5: Run tests**

Run: `go test ./cli/cmd/... -run TestInstall -v -race -count=1`

Expected: All PASS.

- [ ] **Step 6: Run full test suite**

Run: `go test ./... -race -count=1`

Expected: All PASS.

- [ ] **Step 7: Commit**

```bash
git add cli/cmd/install.go cli/cmd/install_test.go cli/cmd/root.go
git commit -m "feat: add install command with atomic writes"
```

### Task 7: Integration test - publish then install

**Files:**
- Modify: `cli/cmd/install_test.go`

- [ ] **Step 1: Add publish-then-install integration test**

Add to `cli/cmd/install_test.go`:

```go
func TestPublishThenInstall(t *testing.T) {
	content := map[string][]byte{}
	ts := testutil.NewStubServerFull(t, nil, content, nil)

	// Create a skill file to publish
	tmpDir := t.TempDir()
	skillFile := filepath.Join(tmpDir, "my-skill.md")
	os.WriteFile(skillFile, []byte("# My Skill\nPublished content"), 0644)

	// Publish
	var pubBuf bytes.Buffer
	pubRoot := cmd.NewRootCmd()
	pubRoot.SetOut(&pubBuf)
	pubRoot.SetArgs([]string{
		"publish",
		"--name", "my-skill",
		"--version", "1.0.0",
		"--description", "Integration test",
		"--file", skillFile,
		"--api-url", ts.URL,
	})
	if err := pubRoot.Execute(); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !strings.Contains(pubBuf.String(), "Published my-skill@1.0.0") {
		t.Errorf("expected publish success, got:\n%s", pubBuf.String())
	}

	// Now the stub has the skill data from the publish call.
	// For install to work, we need the content in the stub's map.
	// The stub's PublishSkill doesn't actually store content, so we
	// pre-populate the content map used by GetSkillContent.
	content["my-skill"] = []byte("# My Skill\nPublished content")

	// Install
	skillsDir := filepath.Join(tmpDir, "skills")
	var installBuf bytes.Buffer
	installRoot := cmd.NewRootCmd()
	installRoot.SetOut(&installBuf)
	installRoot.SetArgs([]string{
		"install", "my-skill",
		"--api-url", ts.URL,
		"--skills-dir", skillsDir,
	})
	if err := installRoot.Execute(); err != nil {
		t.Fatalf("install: %v", err)
	}
	if !strings.Contains(installBuf.String(), "Installed my-skill@") {
		t.Errorf("expected install success, got:\n%s", installBuf.String())
	}

	// Verify file content
	installed, err := os.ReadFile(filepath.Join(skillsDir, "my-skill.md"))
	if err != nil {
		t.Fatalf("read installed file: %v", err)
	}
	if string(installed) != "# My Skill\nPublished content" {
		t.Errorf("unexpected installed content: %q", string(installed))
	}
}
```

- [ ] **Step 2: Run integration test**

Run: `go test ./cli/cmd/... -run TestPublishThenInstall -v -race -count=1`

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add cli/cmd/install_test.go
git commit -m "test: add publish-then-install integration test"
```

---

## Chunk 5: Final Verification

### Task 8: Linting, full test suite, build verification

**Files:** None modified, verification only.

- [ ] **Step 1: Run full test suite with race detection**

Run: `go test ./... -race -count=1`

Expected: All PASS.

- [ ] **Step 2: Run linter**

Run: `golangci-lint run ./...`

Expected: No new errors from our changes. Fix any goimports issues.

- [ ] **Step 3: Verify both binaries build**

Run: `CGO_ENABLED=0 go build -o /tmp/skillsctl-server ./backend/cmd/server && CGO_ENABLED=0 go build -o /tmp/skillsctl-cli ./cli`

Expected: Both compile successfully.

- [ ] **Step 4: Fix any issues found, commit if needed**

If linting or tests surfaced issues, fix and commit with an appropriate message.
