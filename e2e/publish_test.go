//go:build e2e

package e2e

import (
	"strings"
	"testing"

	"github.com/nebari-dev/skillsctl/e2e/testutil"
)

func TestPublishThenExplore(t *testing.T) {
	r := newRunner(t)
	name := skillName(t, "pub-explore")
	content := "---\nname: test-skill\n---\nThis is e2e test content."

	// Publish a skill.
	testutil.PublishSkill(t, r, name, "1.0.0", content)

	// Verify it appears in explore list.
	res := r.Run("explore")
	if res.ExitCode != 0 {
		t.Fatalf("explore failed (exit %d): %s", res.ExitCode, res.Stderr)
	}
	if !strings.Contains(res.Stdout, name) {
		t.Errorf("explore output should contain %q, got:\n%s", name, res.Stdout)
	}

	// Verify explore show returns correct metadata.
	res = r.Run("explore", "show", name)
	if res.ExitCode != 0 {
		t.Fatalf("explore show failed (exit %d): %s", res.ExitCode, res.Stderr)
	}
	for _, want := range []string{
		"Name:        " + name,
		"Description: e2e test skill",
		"Version:     1.0.0",
	} {
		if !strings.Contains(res.Stdout, want) {
			t.Errorf("explore show should contain %q, got:\n%s", want, res.Stdout)
		}
	}

	// Verify --verbose includes content.
	res = r.Run("explore", "show", name, "--verbose")
	if res.ExitCode != 0 {
		t.Fatalf("explore show --verbose failed (exit %d): %s", res.ExitCode, res.Stderr)
	}
	if !strings.Contains(res.Stdout, content) {
		t.Errorf("verbose output should contain published content, got:\n%s", res.Stdout)
	}
}
