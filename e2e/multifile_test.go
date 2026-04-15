//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPublishInstallMultiFile covers the new skill-resources flow:
// publish a directory with SKILL.md + scripts/run.sh + references/notes.md,
// install it, and assert the extracted tree.
func TestPublishInstallMultiFile(t *testing.T) {
	r := newRunner(t)
	name := skillName(t, "multi")

	src := t.TempDir()
	files := map[string]string{
		"SKILL.md":            "# multi-file skill\n\nContent.\n",
		"scripts/run.sh":      "#!/bin/sh\necho hi\n",
		"references/notes.md": "some notes\n",
	}
	for rel, body := range files {
		full := filepath.Join(src, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	pub := r.Run("publish",
		"--name", name,
		"--version", "1.0.0",
		"--description", "multi-file e2e",
		"--dir", src,
	)
	if pub.ExitCode != 0 {
		t.Fatalf("publish failed (exit %d): %s", pub.ExitCode, pub.Stderr)
	}
	if !strings.Contains(pub.Stdout, "Published "+name+"@1.0.0") {
		t.Errorf("publish output missing confirmation: %s", pub.Stdout)
	}

	ins := r.Run("install", name)
	if ins.ExitCode != 0 {
		t.Fatalf("install failed (exit %d): %s", ins.ExitCode, ins.Stderr)
	}

	for rel, want := range files {
		got, err := os.ReadFile(filepath.Join(r.SkillsDir, name, rel))
		if err != nil {
			t.Errorf("read %s: %v", rel, err)
			continue
		}
		if string(got) != want {
			t.Errorf("%s: got %q, want %q", rel, string(got), want)
		}
	}
}

// TestInstallForceOverwrites covers the new --force flag.
func TestInstallForceOverwrites(t *testing.T) {
	r := newRunner(t)
	name := skillName(t, "force")

	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pub1 := r.Run("publish",
		"--name", name,
		"--version", "1.0.0",
		"--description", "force test",
		"--dir", src,
	)
	if pub1.ExitCode != 0 {
		t.Fatalf("publish v1 failed: %s", pub1.Stderr)
	}

	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pub2 := r.Run("publish",
		"--name", name,
		"--version", "2.0.0",
		"--description", "force test",
		"--dir", src,
	)
	if pub2.ExitCode != 0 {
		t.Fatalf("publish v2 failed: %s", pub2.Stderr)
	}

	// First install - fresh dir.
	first := r.Run("install", name+"@1.0.0")
	if first.ExitCode != 0 {
		t.Fatalf("first install failed: %s", first.Stderr)
	}

	// Second install without --force must fail.
	second := r.Run("install", name+"@2.0.0")
	if second.ExitCode == 0 {
		t.Fatal("second install without --force should fail")
	}

	// --force succeeds and overwrites.
	third := r.Run("install", name+"@2.0.0", "--force")
	if third.ExitCode != 0 {
		t.Fatalf("--force install failed: %s", third.Stderr)
	}

	got, err := os.ReadFile(filepath.Join(r.SkillsDir, name, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "v2\n" {
		t.Errorf("expected v2 after --force, got %q", got)
	}
}
