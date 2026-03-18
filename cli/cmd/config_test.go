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
	root.SetIn(strings.NewReader("\n\n"))
	root.SetArgs([]string{"config", "init", "--config-path", configPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(configPath) //nolint:gosec // test file, path is from t.TempDir()
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
	if err := os.WriteFile(configPath, []byte("api_url: http://example.com\n"), 0644); err != nil { //nolint:gosec // test file
		t.Fatalf("write config: %v", err)
	}

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
	if err := os.WriteFile(configPath, []byte("api_url: http://old.com\n"), 0644); err != nil { //nolint:gosec // test file
		t.Fatalf("write config: %v", err)
	}

	var buf bytes.Buffer
	root := cmd.NewRootCmd()
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetIn(strings.NewReader("http://new.com\n\n"))
	root.SetArgs([]string{"config", "init", "--config-path", configPath, "--force"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(configPath) //nolint:gosec // test file, path is from t.TempDir()
	if !strings.Contains(string(data), "http://new.com") {
		t.Errorf("expected new URL in config, got:\n%s", string(data))
	}
}

func TestConfigSetAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"config", "set", "api_url", "http://custom.com", "--config-path", configPath})
	if err := root.Execute(); err != nil {
		t.Fatalf("set: %v", err)
	}

	data, err := os.ReadFile(configPath) //nolint:gosec // test file, path is from t.TempDir()
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
