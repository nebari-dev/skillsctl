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
