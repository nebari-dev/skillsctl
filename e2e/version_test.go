//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nebari-dev/skillsctl/e2e/testutil"
)

func TestVersionLifecycle(t *testing.T) {
	r := newRunner(t)
	name := skillName(t, "versions")
	contentV1 := "---\nname: version-test\n---\nVersion 1 content."
	contentV2 := "---\nname: version-test\n---\nVersion 2 content."

	// Publish two versions.
	testutil.PublishSkill(t, r, name, "1.0.0", contentV1)
	testutil.PublishSkill(t, r, name, "2.0.0", contentV2)

	// Explore show should report v2 as latest.
	res := r.Run("explore", "show", name)
	if res.ExitCode != 0 {
		t.Fatalf("explore show failed: %s", res.Stderr)
	}
	if !strings.Contains(res.Stdout, "Version:     2.0.0") {
		t.Errorf("latest version should be 2.0.0, got:\n%s", res.Stdout)
	}

	// Install specific version (v1).
	r1 := newRunner(t)
	res = r1.Run("install", name+"@1.0.0")
	if res.ExitCode != 0 {
		t.Fatalf("install v1 failed (exit %d): %s", res.ExitCode, res.Stderr)
	}
	got, err := os.ReadFile(filepath.Join(r1.SkillsDir, name+".md"))
	if err != nil {
		t.Fatalf("read installed v1: %v", err)
	}
	if string(got) != contentV1 {
		t.Errorf("v1 content mismatch:\ngot:  %q\nwant: %q", string(got), contentV1)
	}

	// Install latest (no version) should get v2.
	r2 := newRunner(t)
	res = r2.Run("install", name)
	if res.ExitCode != 0 {
		t.Fatalf("install latest failed (exit %d): %s", res.ExitCode, res.Stderr)
	}
	got, err = os.ReadFile(filepath.Join(r2.SkillsDir, name+".md"))
	if err != nil {
		t.Fatalf("read installed latest: %v", err)
	}
	if string(got) != contentV2 {
		t.Errorf("latest content mismatch:\ngot:  %q\nwant: %q", string(got), contentV2)
	}
}
