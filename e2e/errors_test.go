//go:build e2e

package e2e

import (
	"strings"
	"testing"

	"github.com/nebari-dev/skillsctl/e2e/testutil"
)

func TestDuplicateVersionFails(t *testing.T) {
	r := newRunner(t)
	name := skillName(t, "dup-version")
	content := "---\nname: dup-test\n---\nDuplicate test."

	testutil.PublishSkill(t, r, name, "1.0.0", content)

	// Second publish of same version should fail.
	res := r.Run("publish",
		"--name", name,
		"--version", "1.0.0",
		"--description", "duplicate",
		"--file", "/dev/null",
	)
	if res.ExitCode == 0 {
		t.Fatal("duplicate version publish should fail with non-zero exit")
	}
}

func TestInstallNonexistentSkill(t *testing.T) {
	r := newRunner(t)

	res := r.Run("install", "e2e-does-not-exist")
	if res.ExitCode == 0 {
		t.Fatal("install nonexistent skill should fail with non-zero exit")
	}
	// Verify a useful error message is present.
	combined := res.Stdout + res.Stderr
	if !strings.Contains(strings.ToLower(combined), "not found") {
		t.Errorf("error output should mention 'not found', got:\nstdout: %s\nstderr: %s",
			res.Stdout, res.Stderr)
	}
}

func TestPublishInvalidName(t *testing.T) {
	r := newRunner(t)

	res := r.Run("publish",
		"--name", "INVALID-UPPERCASE",
		"--version", "1.0.0",
		"--description", "bad name",
		"--file", "/dev/null",
	)
	if res.ExitCode == 0 {
		t.Fatal("publish with invalid name should fail with non-zero exit")
	}
}
