//go:build e2e

package testutil

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// PublishSkill publishes a skill via the CLI binary. It creates a temp
// directory containing SKILL.md with the given content and runs `publish --dir`.
// Fails the test on non-zero exit. Returns the Result for digest extraction.
func PublishSkill(t *testing.T, r *CLIRunner, name, version, content string) *Result {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	res := r.Run("publish",
		"--name", name,
		"--version", version,
		"--description", "e2e test skill",
		"--dir", dir,
	)
	if res.ExitCode != 0 {
		t.Fatalf("publish %s@%s failed (exit %d):\nstdout: %s\nstderr: %s",
			name, version, res.ExitCode, res.Stdout, res.Stderr)
	}
	return res
}

var digestRe = regexp.MustCompile(`\(sha256:[0-9a-f]+\)`)

// ExtractDigest parses the digest from publish output.
// Expected format: "Published name@version (sha256:abc123...)\n"
func ExtractDigest(t *testing.T, publishOutput string) string {
	t.Helper()
	match := digestRe.FindString(publishOutput)
	if match == "" {
		t.Fatalf("no digest found in publish output: %s", publishOutput)
	}
	// Strip surrounding parens.
	return match[1 : len(match)-1]
}
