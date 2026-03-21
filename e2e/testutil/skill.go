//go:build e2e

package testutil

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// PublishSkill publishes a skill via the CLI binary and fails the test if
// the command exits non-zero. Returns the Result so callers can parse the
// publish output (e.g., to extract the digest from "Published name@version (sha256:...)").
func PublishSkill(t *testing.T, r *CLIRunner, name, version, content string) *Result {
	t.Helper()

	tmpFile := filepath.Join(t.TempDir(), name+".md")
	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp skill file: %v", err)
	}

	res := r.Run("publish",
		"--name", name,
		"--version", version,
		"--description", "e2e test skill",
		"--file", tmpFile,
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
