//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nebari-dev/skillsctl/e2e/testutil"
)

func TestPublishThenInstall(t *testing.T) {
	r := newRunner(t)
	name := skillName(t, "pub-install")
	content := "---\nname: install-test\n---\nInstall e2e content."

	testutil.PublishSkill(t, r, name, "1.0.0", content)

	// Install and verify file lands on disk.
	res := r.Run("install", name)
	if res.ExitCode != 0 {
		t.Fatalf("install failed (exit %d): %s", res.ExitCode, res.Stderr)
	}
	if !strings.Contains(res.Stdout, "Installed "+name+"@1.0.0") {
		t.Errorf("install output should confirm installation, got:\n%s", res.Stdout)
	}

	installed := filepath.Join(r.SkillsDir, name+".md")
	got, err := os.ReadFile(installed)
	if err != nil {
		t.Fatalf("read installed file: %v", err)
	}
	if string(got) != content {
		t.Errorf("installed content mismatch:\ngot:  %q\nwant: %q", string(got), content)
	}
}

func TestInstallWithCorrectDigest(t *testing.T) {
	r := newRunner(t)
	name := skillName(t, "digest-ok")
	content := "---\nname: digest-test\n---\nDigest e2e content."

	// Capture publish output to extract digest.
	// Output format: "Published <name>@<version> (<digest>)\n"
	pubRes := testutil.PublishSkill(t, r, name, "1.0.0", content)
	digest := testutil.ExtractDigest(t, pubRes.Stdout)

	// Install with correct digest should succeed.
	r2 := newRunner(t)
	res := r2.Run("install", name, "--digest", digest)
	if res.ExitCode != 0 {
		t.Fatalf("install with correct digest failed (exit %d): %s", res.ExitCode, res.Stderr)
	}

	installed := filepath.Join(r2.SkillsDir, name+".md")
	got, err := os.ReadFile(installed)
	if err != nil {
		t.Fatalf("read installed file: %v", err)
	}
	if string(got) != content {
		t.Errorf("installed content mismatch:\ngot:  %q\nwant: %q", string(got), content)
	}
}

func TestInstallWithWrongDigest(t *testing.T) {
	r := newRunner(t)
	name := skillName(t, "digest-bad")
	content := "---\nname: bad-digest\n---\nBad digest content."

	testutil.PublishSkill(t, r, name, "1.0.0", content)

	// Install with wrong digest should fail.
	res := r.Run("install", name, "--digest", "sha256:0000000000000000000000000000000000000000000000000000000000000000")
	if res.ExitCode == 0 {
		t.Fatal("install with wrong digest should fail with non-zero exit")
	}

	// Verify no file was written.
	installed := filepath.Join(r.SkillsDir, name+".md")
	if _, err := os.Stat(installed); err == nil {
		t.Error("file should not exist after failed digest verification")
	}
}
