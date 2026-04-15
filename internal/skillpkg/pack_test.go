package skillpkg

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name string, body []byte) {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, body, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestPackDeterministic(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "SKILL.md", []byte("# skill\n"))
	writeFile(t, dir, "scripts/run.sh", []byte("echo hi\n"))
	writeFile(t, dir, "references/notes.md", []byte("notes\n"))

	a, err := Pack(dir)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	b, err := Pack(dir)
	if err != nil {
		t.Fatalf("Pack 2: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("Pack is not deterministic across runs")
	}
}

func TestPackRequiresSkillMd(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "other.md", []byte("x"))
	if _, err := Pack(dir); err == nil {
		t.Fatal("want error for missing SKILL.md")
	}
}

func TestPackRejectsSymlinks(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "SKILL.md", []byte("x"))
	if err := os.Symlink("SKILL.md", filepath.Join(dir, "link")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	if _, err := Pack(dir); err == nil {
		t.Fatal("want error for symlink in source dir")
	}
}

func TestPackOutputValidates(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "SKILL.md", []byte("# x\n"))
	writeFile(t, dir, "a/b.txt", []byte("hello"))

	tgz, err := Pack(dir)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if err := Validate(tgz, DefaultLimits()); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}
