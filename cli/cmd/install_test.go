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

	installed, err := os.ReadFile(filepath.Join(skillsDir, "my-skill", "SKILL.md")) //nolint:gosec // test file, path is from t.TempDir()
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

	if _, err := os.Stat(filepath.Join(skillsDir, "my-skill", "SKILL.md")); err != nil {
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

func TestInstall_ProjectFlag(t *testing.T) {
	content := map[string][]byte{
		"my-skill": []byte("# My Skill"),
	}
	ts := testutil.NewStubServerWithContent(t, testutil.SeedSkills(), content)

	projectDir := t.TempDir()

	var buf bytes.Buffer
	root := cmd.NewRootCmd()
	root.SetOut(&buf)
	root.SetArgs([]string{
		"install", "my-skill",
		"--api-url", ts.URL,
		"--project=" + projectDir,
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := filepath.Join(projectDir, ".claude", "skills", "my-skill", "SKILL.md")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("skill file not created at %s: %v", want, err)
	}
	if !strings.Contains(buf.String(), want) {
		t.Errorf("expected install path %q in output, got:\n%s", want, buf.String())
	}
}

func TestInstall_ProjectFlagBareUsesCWD(t *testing.T) {
	content := map[string][]byte{
		"my-skill": []byte("# My Skill"),
	}
	ts := testutil.NewStubServerWithContent(t, testutil.SeedSkills(), content)

	projectDir := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	root := cmd.NewRootCmd()
	root.SetArgs([]string{
		"install", "my-skill",
		"--api-url", ts.URL,
		"--project",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(projectDir, ".claude", "skills", "my-skill", "SKILL.md")); err != nil {
		t.Fatalf("skill file not created in CWD project: %v", err)
	}
}

func TestInstall_ProjectAndSkillsDirConflict(t *testing.T) {
	ts := testutil.NewStubServer(t, testutil.SeedSkills())

	tmpDir := t.TempDir()

	root := cmd.NewRootCmd()
	root.SetArgs([]string{
		"install", "my-skill",
		"--api-url", ts.URL,
		"--project=" + tmpDir,
		"--skills-dir", tmpDir,
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when both --project and --skills-dir are set")
	}
	if !strings.Contains(err.Error(), "project") || !strings.Contains(err.Error(), "skills-dir") {
		t.Errorf("expected error mentioning both flags, got: %v", err)
	}
	if !strings.Contains(err.Error(), "none of the others can be") {
		t.Errorf("expected Cobra mutual-exclusion error, got: %v", err)
	}
}

func TestPublishThenInstall(t *testing.T) {
	content := map[string][]byte{}
	ts := testutil.NewStubServerFull(t, nil, content, nil)

	// Create a skill directory to publish
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skill-src")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# My Skill\nPublished content"), 0o644); err != nil { //nolint:gosec // test file
		t.Fatalf("write SKILL.md: %v", err)
	}

	// Publish
	var pubBuf bytes.Buffer
	pubRoot := cmd.NewRootCmd()
	pubRoot.SetOut(&pubBuf)
	pubRoot.SetArgs([]string{
		"publish",
		"--name", "my-skill",
		"--version", "1.0.0",
		"--description", "Integration test",
		"--dir", skillDir,
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
	installed, err := os.ReadFile(filepath.Join(skillsDir, "my-skill", "SKILL.md")) //nolint:gosec // test file, path is from t.TempDir()
	if err != nil {
		t.Fatalf("read installed file: %v", err)
	}
	if string(installed) != "# My Skill\nPublished content" {
		t.Errorf("unexpected installed content: %q", string(installed))
	}
}
