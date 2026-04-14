package skillpkg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractRoundTrip(t *testing.T) {
	src := t.TempDir()
	writeFile(t, src, "SKILL.md", []byte("# x\n"))
	writeFile(t, src, "scripts/run.sh", []byte("echo\n"))
	writeFile(t, src, "references/notes/a.md", []byte("a\n"))

	tgz, err := Pack(src)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "out")
	if err := Extract(tgz, dst, DefaultLimits()); err != nil {
		t.Fatalf("Extract: %v", err)
	}

	for _, rel := range []string{"SKILL.md", "scripts/run.sh", "references/notes/a.md"} {
		got, err := os.ReadFile(filepath.Join(dst, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		want, _ := os.ReadFile(filepath.Join(src, rel))
		if string(got) != string(want) {
			t.Errorf("%s: got %q want %q", rel, got, want)
		}
	}
}

func TestExtractRefusesExistingDest(t *testing.T) {
	src := t.TempDir()
	writeFile(t, src, "SKILL.md", []byte("x"))
	tgz, _ := Pack(src)

	dst := t.TempDir() // exists
	err := Extract(tgz, dst, DefaultLimits())
	if err == nil {
		t.Fatal("want error when destination already exists")
	}
}

func TestExtractValidates(t *testing.T) {
	// Build a tarball with a symlink and verify Extract refuses it.
	tgz := buildTarGz(t, []tarEntry{
		{name: "SKILL.md", mode: 0o644, body: []byte("x")},
		{name: "link", typeflag: 0x32 /* tar.TypeSymlink */, linkname: "SKILL.md"},
	})
	dst := filepath.Join(t.TempDir(), "out")
	if err := Extract(tgz, dst, DefaultLimits()); err == nil {
		t.Fatal("want error for symlink entry")
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("dst should not exist, got err=%v", err)
	}
}
