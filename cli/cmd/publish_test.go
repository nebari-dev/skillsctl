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

func writeSkillDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# My Skill\nDoes stuff\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	return dir
}

func TestPublish(t *testing.T) {
	ts := testutil.NewStubServer(t, nil)
	dir := writeSkillDir(t)

	var buf bytes.Buffer
	root := cmd.NewRootCmd()
	root.SetOut(&buf)
	root.SetArgs([]string{
		"publish",
		"--name", "my-skill",
		"--version", "1.0.0",
		"--description", "A test skill",
		"--dir", dir,
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

func TestPublish_DirNotFound(t *testing.T) {
	ts := testutil.NewStubServer(t, nil)

	root := cmd.NewRootCmd()
	root.SetArgs([]string{
		"publish",
		"--name", "my-skill",
		"--version", "1.0.0",
		"--description", "desc",
		"--dir", "/nonexistent/dir",
		"--api-url", ts.URL,
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestPublish_MissingSkillMd(t *testing.T) {
	ts := testutil.NewStubServer(t, nil)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "other.md"), []byte("nope"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	root := cmd.NewRootCmd()
	root.SetArgs([]string{
		"publish",
		"--name", "my-skill",
		"--version", "1.0.0",
		"--description", "desc",
		"--dir", dir,
		"--api-url", ts.URL,
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing SKILL.md")
	}
	if !strings.Contains(err.Error(), "SKILL.md") {
		t.Errorf("expected error mentioning SKILL.md, got: %v", err)
	}
}

func TestPublish_AlreadyExists(t *testing.T) {
	ts := testutil.NewStubServerFull(t, nil, nil,
		connect.NewError(connect.CodeAlreadyExists, nil))
	dir := writeSkillDir(t)

	var buf bytes.Buffer
	root := cmd.NewRootCmd()
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{
		"publish",
		"--name", "my-skill",
		"--version", "1.0.0",
		"--description", "desc",
		"--dir", dir,
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
