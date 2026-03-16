package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nebari-dev/skillctl/cli/cmd"
	"github.com/nebari-dev/skillctl/cli/internal/testutil"
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

	// The stub's PublishSkill doesn't store content, so populate
	// the content map used by GetSkillContent.
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
