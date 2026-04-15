# Skill Resources Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users publish and install Claude Code skills as directories (SKILL.md plus supporting files) instead of single files, with no proto or DB schema changes.

**Architecture:** A new shared package `internal/skillpkg` packs a directory into a deterministic `tar.gz`, validates tarballs against configurable limits, and extracts them safely. Tarballs ride through the existing `PublishSkillRequest.content` bytes field and live in the existing `skill_versions.content` BLOB. The CLI fetches limits from a new unauthenticated `GET /limits` endpoint. Install detects gzip magic so legacy single-file rows still install as today.

**Tech Stack:** Go 1.25, ConnectRPC, Cobra/Viper, modernc.org/sqlite. Stdlib `archive/tar` and `compress/gzip` for packing.

**Spec:** `docs/superpowers/specs/2026-04-14-skill-resources-design.md`

---

## File Structure

**Create:**
- `internal/skillpkg/limits.go` - `Limits` struct + defaults
- `internal/skillpkg/pack.go` - `Pack(dir string) ([]byte, error)`
- `internal/skillpkg/validate.go` - `Validate`, `IsTarball`
- `internal/skillpkg/extract.go` - `Extract(tarball, destDir, limits)`
- `internal/skillpkg/*_test.go` - table-driven tests for each
- `backend/internal/server/limits.go` - `GET /limits` handler
- `cli/internal/api/limits.go` - client method for `GET /limits`

**Modify:**
- `backend/internal/registry/service.go` - call `skillpkg.Validate` in `PublishSkill`
- `backend/internal/registry/validate.go` - drop `maxContentBytes`; let `skillpkg` own packed-byte cap
- `backend/internal/server/server.go` - mount `/limits`, allowlist it
- `backend/cmd/server/main.go` - load `Limits` from Viper, pass into `server.New`
- `cli/cmd/publish.go` - replace `--file` with `--dir`, call `skillpkg.Pack`, fetch limits
- `cli/cmd/install.go` - branch on `IsTarball`, add `--force`, extract via `skillpkg.Extract`
- `cli/internal/api/client.go` - add limits client method (or new file above)

---

## Task 1: Define Limits and defaults

**Files:**
- Create: `internal/skillpkg/limits.go`
- Create: `internal/skillpkg/limits_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/skillpkg/limits_test.go
package skillpkg

import "testing"

func TestDefaultLimits(t *testing.T) {
	l := DefaultLimits()
	if l.MaxPackedBytes != 5*1024*1024 {
		t.Errorf("MaxPackedBytes = %d, want %d", l.MaxPackedBytes, 5*1024*1024)
	}
	if l.MaxTotalBytes != 20*1024*1024 {
		t.Errorf("MaxTotalBytes = %d, want %d", l.MaxTotalBytes, 20*1024*1024)
	}
	if l.MaxFiles != 100 {
		t.Errorf("MaxFiles = %d, want 100", l.MaxFiles)
	}
	if l.MaxFileBytes != 1024*1024 {
		t.Errorf("MaxFileBytes = %d, want %d", l.MaxFileBytes, 1024*1024)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/skillpkg/...`
Expected: FAIL with "package internal/skillpkg: no Go files" or undefined.

- [ ] **Step 3: Implement Limits**

```go
// internal/skillpkg/limits.go
package skillpkg

// Limits caps the size and shape of a skill tarball. Zero values mean unlimited.
type Limits struct {
	MaxPackedBytes int64 // compressed tarball size
	MaxTotalBytes  int64 // sum of uncompressed file sizes
	MaxFiles       int   // file count (excludes directories)
	MaxFileBytes   int64 // largest single uncompressed file
}

// DefaultLimits returns the built-in defaults used when the server has not
// configured limits and the CLI cannot reach GET /limits.
func DefaultLimits() Limits {
	return Limits{
		MaxPackedBytes: 5 * 1024 * 1024,
		MaxTotalBytes:  20 * 1024 * 1024,
		MaxFiles:       100,
		MaxFileBytes:   1024 * 1024,
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/skillpkg/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/skillpkg/limits.go internal/skillpkg/limits_test.go
git commit -m "feat(skillpkg): add Limits with defaults"
```

---

## Task 2: IsTarball detection

**Files:**
- Create: `internal/skillpkg/validate.go`
- Create: `internal/skillpkg/validate_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/skillpkg/validate_test.go
package skillpkg

import "testing"

func TestIsTarball(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want bool
	}{
		{"gzip magic", []byte{0x1f, 0x8b, 0x08, 0x00}, true},
		{"plain markdown", []byte("# SKILL\n"), false},
		{"empty", nil, false},
		{"one byte", []byte{0x1f}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTarball(tt.in); got != tt.want {
				t.Errorf("IsTarball(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/skillpkg/ -run TestIsTarball`
Expected: FAIL, undefined IsTarball.

- [ ] **Step 3: Implement IsTarball**

```go
// internal/skillpkg/validate.go
package skillpkg

// IsTarball reports whether b begins with the gzip magic bytes. We use this to
// distinguish multi-file skill tarballs from legacy raw SKILL.md content.
func IsTarball(b []byte) bool {
	return len(b) >= 2 && b[0] == 0x1f && b[1] == 0x8b
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/skillpkg/ -run TestIsTarball`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/skillpkg/validate.go internal/skillpkg/validate_test.go
git commit -m "feat(skillpkg): add IsTarball gzip-magic check"
```

---

## Task 3: Validate tarballs

**Files:**
- Modify: `internal/skillpkg/validate.go`
- Modify: `internal/skillpkg/validate_test.go`

- [ ] **Step 1: Add the failing tests**

Append to `internal/skillpkg/validate_test.go`:

```go
import (
	"archive/tar"
	"bytes"
	"compress/gzip"
)

// helper to build a tar.gz from a list of (name, mode, body) entries.
func buildTarGz(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for _, e := range entries {
		hdr := &tar.Header{
			Name:     e.name,
			Mode:     e.mode,
			Size:     int64(len(e.body)),
			Typeflag: e.typeflag,
			Linkname: e.linkname,
		}
		if e.typeflag == 0 {
			hdr.Typeflag = tar.TypeReg
		}
		if hdr.Typeflag == tar.TypeDir {
			hdr.Size = 0
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader: %v", err)
		}
		if hdr.Typeflag == tar.TypeReg {
			if _, err := tw.Write(e.body); err != nil {
				t.Fatalf("Write: %v", err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tw.Close: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gw.Close: %v", err)
	}
	return buf.Bytes()
}

type tarEntry struct {
	name     string
	mode     int64
	body     []byte
	typeflag byte
	linkname string
}

func TestValidate(t *testing.T) {
	good := []tarEntry{{name: "SKILL.md", mode: 0o644, body: []byte("# x\n")}}
	limits := Limits{MaxPackedBytes: 1 << 20, MaxTotalBytes: 1 << 20, MaxFiles: 10, MaxFileBytes: 1 << 16}

	tests := []struct {
		name    string
		entries []tarEntry
		wantErr string
	}{
		{"happy path", good, ""},
		{"missing SKILL.md", []tarEntry{{name: "other.md", mode: 0o644, body: []byte("x")}}, "SKILL.md"},
		{"absolute path", []tarEntry{{name: "/SKILL.md", mode: 0o644, body: []byte("x")}}, "absolute"},
		{"dotdot", []tarEntry{{name: "../SKILL.md", mode: 0o644, body: []byte("x")}}, ".."},
		{"symlink rejected", []tarEntry{
			{name: "SKILL.md", mode: 0o644, body: []byte("x")},
			{name: "link", typeflag: tar.TypeSymlink, linkname: "SKILL.md"},
		}, "symlink"},
		{"hardlink rejected", []tarEntry{
			{name: "SKILL.md", mode: 0o644, body: []byte("x")},
			{name: "hl", typeflag: tar.TypeLink, linkname: "SKILL.md"},
		}, "hardlink"},
		{"too many files", func() []tarEntry {
			es := []tarEntry{{name: "SKILL.md", mode: 0o644, body: []byte("x")}}
			for i := 0; i < 11; i++ {
				es = append(es, tarEntry{name: "f" + string(rune('a'+i)), mode: 0o644, body: []byte("x")})
			}
			return es
		}(), "too many files"},
		{"file too big", []tarEntry{
			{name: "SKILL.md", mode: 0o644, body: bytes.Repeat([]byte("x"), (1<<16)+1)},
		}, "file size"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tgz := buildTarGz(t, tt.entries)
			err := Validate(tgz, limits)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !bytes.Contains([]byte(err.Error()), []byte(tt.wantErr)) {
				t.Fatalf("got %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePackedTooLarge(t *testing.T) {
	entries := []tarEntry{{name: "SKILL.md", mode: 0o644, body: []byte("x")}}
	tgz := buildTarGz(t, entries)
	limits := Limits{MaxPackedBytes: 1, MaxTotalBytes: 1 << 20, MaxFiles: 10, MaxFileBytes: 1 << 20}
	if err := Validate(tgz, limits); err == nil || !bytes.Contains([]byte(err.Error()), []byte("packed")) {
		t.Fatalf("got %v, want packed-size error", err)
	}
}

func TestValidateTotalTooLarge(t *testing.T) {
	entries := []tarEntry{
		{name: "SKILL.md", mode: 0o644, body: bytes.Repeat([]byte("a"), 600)},
		{name: "b", mode: 0o644, body: bytes.Repeat([]byte("b"), 600)},
	}
	tgz := buildTarGz(t, entries)
	limits := Limits{MaxPackedBytes: 1 << 20, MaxTotalBytes: 1000, MaxFiles: 10, MaxFileBytes: 1 << 20}
	if err := Validate(tgz, limits); err == nil || !bytes.Contains([]byte(err.Error()), []byte("total")) {
		t.Fatalf("got %v, want total-size error", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/skillpkg/ -run TestValidate`
Expected: FAIL, undefined Validate.

- [ ] **Step 3: Implement Validate**

```go
// internal/skillpkg/validate.go
package skillpkg

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
)

// Validate inspects a gzip-compressed tar archive and enforces structural and
// size rules. It does not write any data; callers can use it as a pre-flight
// check before storing or extracting.
func Validate(tarball []byte, limits Limits) error {
	if limits.MaxPackedBytes > 0 && int64(len(tarball)) > limits.MaxPackedBytes {
		return fmt.Errorf("tarball packed size %d exceeds limit %d", len(tarball), limits.MaxPackedBytes)
	}
	gr, err := gzip.NewReader(bytes.NewReader(tarball))
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	var (
		fileCount   int
		totalBytes  int64
		hasSkillMd  bool
	)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		if err := checkHeader(hdr); err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		fileCount++
		if limits.MaxFiles > 0 && fileCount > limits.MaxFiles {
			return fmt.Errorf("too many files (limit %d)", limits.MaxFiles)
		}
		if limits.MaxFileBytes > 0 && hdr.Size > limits.MaxFileBytes {
			return fmt.Errorf("file %q size %d exceeds per-file limit %d", hdr.Name, hdr.Size, limits.MaxFileBytes)
		}
		totalBytes += hdr.Size
		if limits.MaxTotalBytes > 0 && totalBytes > limits.MaxTotalBytes {
			return fmt.Errorf("total uncompressed size exceeds limit %d", limits.MaxTotalBytes)
		}
		// Drain the entry body to advance the tar reader and detect short reads.
		if _, err := io.Copy(io.Discard, tr); err != nil {
			return fmt.Errorf("read %q: %w", hdr.Name, err)
		}
		if hdr.Name == "SKILL.md" {
			hasSkillMd = true
		}
	}
	if !hasSkillMd {
		return errors.New("SKILL.md missing at tarball root")
	}
	return nil
}

func checkHeader(hdr *tar.Header) error {
	switch hdr.Typeflag {
	case tar.TypeSymlink:
		return fmt.Errorf("symlink %q not allowed", hdr.Name)
	case tar.TypeLink:
		return fmt.Errorf("hardlink %q not allowed", hdr.Name)
	case tar.TypeReg, tar.TypeDir:
		// ok
	default:
		return fmt.Errorf("unsupported tar entry type %c for %q", hdr.Typeflag, hdr.Name)
	}
	name := hdr.Name
	if strings.HasPrefix(name, "/") {
		return fmt.Errorf("absolute path %q not allowed", name)
	}
	clean := path.Clean(name)
	if clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains("/"+clean+"/", "/../") {
		return fmt.Errorf("path %q contains ..", name)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/skillpkg/ -run TestValidate -race`
Expected: PASS for all subtests.

- [ ] **Step 5: Commit**

```bash
git add internal/skillpkg/validate.go internal/skillpkg/validate_test.go
git commit -m "feat(skillpkg): validate tarball structure and size limits"
```

---

## Task 4: Pack a directory deterministically

**Files:**
- Create: `internal/skillpkg/pack.go`
- Create: `internal/skillpkg/pack_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/skillpkg/pack_test.go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/skillpkg/ -run TestPack`
Expected: FAIL, undefined Pack.

- [ ] **Step 3: Implement Pack**

```go
// internal/skillpkg/pack.go
package skillpkg

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Pack walks dir and produces a deterministic gzip-compressed tar archive.
// Output is byte-identical for the same input across runs and hosts:
// entries are sorted by tar name, mtime is zeroed, uid/gid are zeroed,
// uname/gname are empty, and modes are normalized (files 0644, dirs 0755).
//
// Pack rejects symlinks and any entry whose relative path escapes dir.
// SKILL.md must exist at the root.
func Pack(dir string) ([]byte, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("abs: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", abs, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", abs)
	}

	type entry struct {
		rel  string
		path string
		info fs.FileInfo
	}
	var entries []entry
	err = filepath.WalkDir(abs, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if p == abs {
			return nil
		}
		rel, err := filepath.Rel(abs, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "../") || rel == ".." {
			return fmt.Errorf("path %q escapes source dir", rel)
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink %q not allowed", rel)
		}
		if !fi.Mode().IsRegular() && !fi.IsDir() {
			return fmt.Errorf("unsupported file type for %q", rel)
		}
		entries = append(entries, entry{rel: rel, path: p, info: fi})
		return nil
	})
	if err != nil {
		return nil, err
	}

	hasSkillMd := false
	for _, e := range entries {
		if e.rel == "SKILL.md" && e.info.Mode().IsRegular() {
			hasSkillMd = true
			break
		}
	}
	if !hasSkillMd {
		return nil, errors.New("SKILL.md missing at root of source directory")
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].rel < entries[j].rel })

	var buf bytes.Buffer
	gw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("gzip writer: %w", err)
	}
	gw.ModTime = zeroTime()
	gw.Name = ""
	tw := tar.NewWriter(gw)

	for _, e := range entries {
		hdr := &tar.Header{
			Name:    e.rel,
			ModTime: zeroTime(),
			Uid:     0,
			Gid:     0,
			Uname:   "",
			Gname:   "",
			Format:  tar.FormatPAX,
		}
		if e.info.IsDir() {
			hdr.Typeflag = tar.TypeDir
			hdr.Name = e.rel + "/"
			hdr.Mode = 0o755
		} else {
			hdr.Typeflag = tar.TypeReg
			hdr.Mode = 0o644
			hdr.Size = e.info.Size()
		}
		// PAX headers carry their own mtime; clear extras to keep output stable.
		hdr.PAXRecords = nil
		hdr.AccessTime = zeroTime()
		hdr.ChangeTime = zeroTime()
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, fmt.Errorf("write header %q: %w", e.rel, err)
		}
		if hdr.Typeflag == tar.TypeReg {
			f, err := os.Open(e.path) //nolint:gosec // path comes from WalkDir under user-supplied dir
			if err != nil {
				return nil, fmt.Errorf("open %q: %w", e.rel, err)
			}
			if _, err := io.Copy(tw, f); err != nil {
				_ = f.Close()
				return nil, fmt.Errorf("copy %q: %w", e.rel, err)
			}
			_ = f.Close()
		}
	}
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("tar close: %w", err)
	}
	if err := gw.Close(); err != nil {
		return nil, fmt.Errorf("gzip close: %w", err)
	}
	return buf.Bytes(), nil
}

func zeroTime() (t timeZero) { return }

type timeZero = struct {
	// alias to time.Time would force an import; use empty struct fallback.
}
```

Note: the placeholder `zeroTime` above will not compile. Replace it with the real implementation in Step 3a:

- [ ] **Step 3a: Replace `zeroTime` with real time import**

In `internal/skillpkg/pack.go`, add `"time"` to the imports, delete the `timeZero` alias and bogus `zeroTime`, and add at the bottom:

```go
func zeroTime() time.Time { return time.Unix(0, 0).UTC() }
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/skillpkg/ -race`
Expected: PASS for all Pack and Validate tests.

- [ ] **Step 5: Commit**

```bash
git add internal/skillpkg/pack.go internal/skillpkg/pack_test.go
git commit -m "feat(skillpkg): deterministic tar.gz packer for skill directories"
```

---

## Task 5: Extract a tarball safely

**Files:**
- Create: `internal/skillpkg/extract.go`
- Create: `internal/skillpkg/extract_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/skillpkg/extract_test.go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/skillpkg/ -run TestExtract`
Expected: FAIL, undefined Extract.

- [ ] **Step 3: Implement Extract**

```go
// internal/skillpkg/extract.go
package skillpkg

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Extract unpacks a gzip-compressed tar archive into destDir. It first
// validates the entire tarball, then writes files into a sibling temp
// directory and renames it into place. destDir must not already exist; the
// caller is responsible for any --force semantics.
func Extract(tarball []byte, destDir string, limits Limits) error {
	if _, err := os.Stat(destDir); err == nil {
		return fmt.Errorf("destination %s already exists", destDir)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", destDir, err)
	}

	if err := Validate(tarball, limits); err != nil {
		return err
	}

	parent := filepath.Dir(destDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", parent, err)
	}
	tmp, err := os.MkdirTemp(parent, ".skillsctl-tmp-")
	if err != nil {
		return fmt.Errorf("mkdtemp: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(tmp)
		}
	}()

	if err := extractInto(tarball, tmp, limits); err != nil {
		return err
	}
	if err := os.Rename(tmp, destDir); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmp, destDir, err)
	}
	cleanup = false
	return nil
}

func extractInto(tarball []byte, destDir string, limits Limits) error {
	gr, err := gzip.NewReader(bytes.NewReader(tarball))
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		if err := checkHeader(hdr); err != nil {
			return err
		}
		full := filepath.Join(destDir, filepath.FromSlash(hdr.Name))
		// Defense in depth: ensure full stays within destDir after symlink-free join.
		rel, err := filepath.Rel(destDir, full)
		if err != nil || rel == ".." || (len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator)) {
			return fmt.Errorf("entry %q escapes destination", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(full, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", full, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", filepath.Dir(full), err)
			}
			if limits.MaxFileBytes > 0 && hdr.Size > limits.MaxFileBytes {
				return fmt.Errorf("file %q exceeds per-file limit", hdr.Name)
			}
			f, err := os.OpenFile(full, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
			if err != nil {
				return fmt.Errorf("create %s: %w", full, err)
			}
			n, copyErr := io.Copy(f, tr)
			closeErr := f.Close()
			if copyErr != nil {
				return fmt.Errorf("write %s: %w", full, copyErr)
			}
			if closeErr != nil {
				return fmt.Errorf("close %s: %w", full, closeErr)
			}
			if n != hdr.Size {
				return fmt.Errorf("short write for %s", full)
			}
		}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/skillpkg/ -race`
Expected: PASS for all skillpkg tests.

- [ ] **Step 5: Commit**

```bash
git add internal/skillpkg/extract.go internal/skillpkg/extract_test.go
git commit -m "feat(skillpkg): extract tarballs via temp dir + rename"
```

---

## Task 6: Server `/limits` endpoint

**Files:**
- Create: `backend/internal/server/limits.go`
- Modify: `backend/internal/server/server.go`
- Modify: `backend/internal/server/server_test.go`

- [ ] **Step 1: Write the failing test**

Append to `backend/internal/server/server_test.go`:

```go
func TestLimitsEndpoint(t *testing.T) {
	limits := skillpkg.Limits{
		MaxPackedBytes: 1234,
		MaxTotalBytes:  5678,
		MaxFiles:       9,
		MaxFileBytes:   1000,
	}
	srv := server.New(memstore.New(), nil, auth.Config{}, server.WithLimits(limits))
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/limits")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var got skillpkg.Limits
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got != limits {
		t.Fatalf("got %+v want %+v", got, limits)
	}
}
```

Add imports as needed: `"github.com/nebari-dev/skillsctl/internal/skillpkg"`, `"encoding/json"`, etc. Mirror existing test wiring (`memstore`, `auth.Config{}`).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./backend/internal/server/ -run TestLimitsEndpoint`
Expected: FAIL: undefined `WithLimits`.

- [ ] **Step 3: Add WithLimits option and `/limits` handler**

Create `backend/internal/server/limits.go`:

```go
package server

import (
	"encoding/json"
	"net/http"

	"github.com/nebari-dev/skillsctl/internal/skillpkg"
)

func handleLimits(l skillpkg.Limits) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(l)
	}
}
```

Modify `backend/internal/server/server.go`:

Add field and option:

```go
type Server struct {
	handler http.Handler
}

type Option func(*serverConfig)

type serverConfig struct {
	limits skillpkg.Limits
}

// WithLimits configures the limits exposed at GET /limits and used to validate
// incoming publish requests.
func WithLimits(l skillpkg.Limits) Option {
	return func(c *serverConfig) { c.limits = l }
}
```

Update `New` signature to accept `opts ...Option`, build a `serverConfig` with `skillpkg.DefaultLimits()` then apply opts, mount `/limits`, add it to the allowlist:

```go
func New(skillStore store.Repository, authValidator auth.TokenValidator, authCfg auth.Config, opts ...Option) *Server {
	cfg := serverConfig{limits: skillpkg.DefaultLimits()}
	for _, o := range opts {
		o(&cfg)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealthz)
	mux.HandleFunc("/auth/config", handleAuthConfig(authCfg))
	mux.HandleFunc("/limits", handleLimits(cfg.limits))

	interceptor := auth.NewInterceptor(authValidator)
	path, handler := skillsctlv1connect.NewRegistryServiceHandler(
		registry.NewService(skillStore, registry.WithLimits(cfg.limits)),
		connect.WithInterceptors(interceptor),
	)
	mux.Handle(path, handler)

	wrapped := auth.NewAllowlistMiddleware([]string{"/healthz", "/auth/config", "/limits"}, mux)
	return &Server{handler: wrapped}
}
```

Note: `registry.WithLimits` is added in Task 7. For this task, omit it from the call until Task 7 lands. Use:

```go
		registry.NewService(skillStore),
```

And revisit in Task 7.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./backend/internal/server/ -race`
Expected: PASS, including pre-existing tests (compatible variadic opts).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/server/limits.go backend/internal/server/server.go backend/internal/server/server_test.go
git commit -m "feat(server): add GET /limits endpoint and WithLimits option"
```

---

## Task 7: Wire skillpkg.Validate into PublishSkill

**Files:**
- Modify: `backend/internal/registry/service.go`
- Modify: `backend/internal/registry/validate.go`
- Modify: `backend/internal/registry/service_test.go`

- [ ] **Step 1: Read the current handler**

Run: `cat backend/internal/registry/service.go`
Note where `validatePublishRequest` is called and how the service struct is constructed.

- [ ] **Step 2: Write the failing test**

Append to `backend/internal/registry/service_test.go`:

```go
func TestPublishSkill_TarballValidated(t *testing.T) {
	// Tarball missing SKILL.md should be rejected with InvalidArgument.
	bad := buildTarGzNoSkillMd(t)
	svc := registry.NewService(memstore.New(), registry.WithLimits(skillpkg.DefaultLimits()))
	_, err := svc.PublishSkill(context.Background(), connect.NewRequest(&skillsctlv1.PublishSkillRequest{
		Name: "demo", Version: "0.1.0", Description: "d", Content: bad,
	}))
	if err == nil {
		t.Fatal("want error")
	}
	var connErr *connect.Error
	if !errors.As(err, &connErr) || connErr.Code() != connect.CodeInvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", err)
	}
}

func TestPublishSkill_LegacySingleFileStillAccepted(t *testing.T) {
	svc := registry.NewService(memstore.New(), registry.WithLimits(skillpkg.DefaultLimits()))
	_, err := svc.PublishSkill(context.Background(), connect.NewRequest(&skillsctlv1.PublishSkillRequest{
		Name: "demo", Version: "0.1.0", Description: "d", Content: []byte("# SKILL.md\n"),
	}))
	if err != nil {
		t.Fatalf("legacy single-file publish failed: %v", err)
	}
}
```

Add a helper at the bottom of the file:

```go
func buildTarGzNoSkillMd(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	hdr := &tar.Header{Name: "other.md", Mode: 0o644, Size: 1, Typeflag: tar.TypeReg}
	_ = tw.WriteHeader(hdr)
	_, _ = tw.Write([]byte("x"))
	_ = tw.Close()
	_ = gw.Close()
	return buf.Bytes()
}
```

Imports: `archive/tar`, `bytes`, `compress/gzip`, `errors`, `connectrpc.com/connect`, `github.com/nebari-dev/skillsctl/internal/skillpkg`.

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./backend/internal/registry/ -run TestPublishSkill_`
Expected: FAIL, undefined `WithLimits`.

- [ ] **Step 4: Add WithLimits option and validation hook**

In `backend/internal/registry/service.go`, add an option pattern:

```go
type Service struct {
	store  store.Repository
	limits skillpkg.Limits
}

type Option func(*Service)

func WithLimits(l skillpkg.Limits) Option {
	return func(s *Service) { s.limits = l }
}

func NewService(repo store.Repository, opts ...Option) *Service {
	s := &Service{store: repo, limits: skillpkg.DefaultLimits()}
	for _, o := range opts {
		o(s)
	}
	return s
}
```

In the `PublishSkill` method, after the existing `validatePublishRequest` call, add:

```go
if skillpkg.IsTarball(req.Msg.Content) {
	if err := skillpkg.Validate(req.Msg.Content, s.limits); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
}
```

(Legacy non-tarball content continues through unchanged - just bytes in the BLOB.)

In `backend/internal/registry/validate.go`, remove `maxContentBytes` and the corresponding length check; the packed-byte cap now lives in `skillpkg.Limits`. Keep the empty-content check.

- [ ] **Step 5: Update server.New to pass limits into registry**

In `backend/internal/server/server.go`, replace `registry.NewService(skillStore)` with `registry.NewService(skillStore, registry.WithLimits(cfg.limits))`.

- [ ] **Step 6: Run all backend tests**

Run: `go test ./backend/... -race`
Expected: PASS, including the two new tests and all pre-existing ones.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/registry/ backend/internal/server/server.go
git commit -m "feat(registry): validate skill tarballs on publish"
```

---

## Task 8: CLI client method for `/limits`

**Files:**
- Create: `cli/internal/api/limits.go`
- Create: `cli/internal/api/limits_test.go`

- [ ] **Step 1: Write the failing test**

```go
// cli/internal/api/limits_test.go
package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nebari-dev/skillsctl/cli/internal/api"
	"github.com/nebari-dev/skillsctl/internal/skillpkg"
)

func TestGetLimits(t *testing.T) {
	want := skillpkg.Limits{MaxPackedBytes: 1, MaxTotalBytes: 2, MaxFiles: 3, MaxFileBytes: 4}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/limits" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(ts.Close)

	c := api.NewClient(ts.URL)
	got, err := c.GetLimits(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cli/internal/api/ -run TestGetLimits`
Expected: FAIL, undefined GetLimits.

- [ ] **Step 3: Implement GetLimits**

```go
// cli/internal/api/limits.go
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nebari-dev/skillsctl/internal/skillpkg"
)

// GetLimits fetches the server's configured tarball limits from /limits.
func (c *Client) GetLimits(ctx context.Context) (skillpkg.Limits, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/limits", nil)
	if err != nil {
		return skillpkg.Limits{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return skillpkg.Limits{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return skillpkg.Limits{}, fmt.Errorf("limits endpoint returned %d", resp.StatusCode)
	}
	var l skillpkg.Limits
	if err := json.NewDecoder(resp.Body).Decode(&l); err != nil {
		return skillpkg.Limits{}, err
	}
	return l, nil
}
```

This requires `Client` to keep its `baseURL`. Modify `cli/internal/api/client.go` `NewClient` to store `baseURL` on the struct:

```go
type Client struct {
	registry skillsctlv1connect.RegistryServiceClient
	token    string
	baseURL  string
}

func NewClient(baseURL string, opts ...ClientOption) *Client {
	c := &Client{baseURL: baseURL}
	// ... rest unchanged
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cli/internal/api/ -race`
Expected: PASS for all api tests.

- [ ] **Step 5: Commit**

```bash
git add cli/internal/api/limits.go cli/internal/api/limits_test.go cli/internal/api/client.go
git commit -m "feat(cli/api): add GetLimits client method"
```

---

## Task 9: CLI publish replaces `--file` with `--dir`

**Files:**
- Modify: `cli/cmd/publish.go`
- Modify: `cli/cmd/publish_test.go`

- [ ] **Step 1: Write the failing test**

Replace existing publish tests' use of `--file` with `--dir`. Add a new test that exercises a temp directory containing SKILL.md and one extra file:

```go
func TestPublishDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scripts/run.sh"), []byte("echo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Use the existing fake server harness in this test file (mirror current pattern).
	srv := newFakeServer(t)

	cmd := newRootCmdForTest(t, srv.URL)
	cmd.SetArgs([]string{"publish",
		"--name", "demo", "--version", "0.1.0",
		"--description", "d", "--dir", dir,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !srv.lastContentIsTarball() {
		t.Fatal("server did not receive a tarball")
	}
}
```

If existing tests in `publish_test.go` use `--file`, update them to use `--dir` with a temp dir containing SKILL.md.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cli/cmd/ -run TestPublish`
Expected: FAIL, `--dir` flag unknown.

- [ ] **Step 3: Replace `--file` with `--dir` in publish.go**

Edit `cli/cmd/publish.go`:

```go
package cmd

import (
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	"github.com/nebari-dev/skillsctl/internal/skillpkg"
)

func addPublishCmd(root *cobra.Command) {
	var (
		name        string
		version     string
		description string
		dirPath     string
		tags        []string
		changelog   string
	)

	publishCmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish a skill to the registry",
		RunE: func(cmd *cobra.Command, _ []string) error {
			content, err := skillpkg.Pack(dirPath)
			if err != nil {
				return fmt.Errorf("pack %s: %w", dirPath, err)
			}

			client := getClientCtx(cmd.Context())

			limits, err := client.GetLimits(cmd.Context())
			if err != nil {
				limits = skillpkg.DefaultLimits()
			}
			if limits.MaxPackedBytes > 0 && int64(len(content)) > limits.MaxPackedBytes {
				return fmt.Errorf("packed skill is %d bytes, server limit is %d", len(content), limits.MaxPackedBytes)
			}

			_, ver, err := client.PublishSkill(cmd.Context(), name, version, description, changelog, tags, content)
			if err != nil {
				return mapPublishError(err, name, version)
			}

			if ver.Digest != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Published %s@%s (%s)\n", name, version, ver.Digest)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Published %s@%s\n", name, version)
			}
			return nil
		},
	}

	publishCmd.Flags().StringVar(&name, "name", "", "Skill name")
	publishCmd.Flags().StringVar(&version, "version", "", "Skill version (semver)")
	publishCmd.Flags().StringVar(&description, "description", "", "Skill description")
	publishCmd.Flags().StringVar(&dirPath, "dir", "", "Path to skill directory containing SKILL.md")
	publishCmd.Flags().StringSliceVar(&tags, "tag", nil, "Tags (repeatable)")
	publishCmd.Flags().StringVar(&changelog, "changelog", "", "Version changelog")

	_ = publishCmd.MarkFlagRequired("name")
	_ = publishCmd.MarkFlagRequired("version")
	_ = publishCmd.MarkFlagRequired("description")
	_ = publishCmd.MarkFlagRequired("dir")

	root.AddCommand(publishCmd)
}

// mapPublishError unchanged, keep existing implementation below.
```

(Keep the existing `mapPublishError` function as-is.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cli/cmd/ -race -run TestPublish`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cli/cmd/publish.go cli/cmd/publish_test.go
git commit -m "feat(cli): publish takes --dir and packs into a tarball"
```

---

## Task 10: CLI install detects tarballs, adds `--force`

**Files:**
- Modify: `cli/cmd/install.go`
- Modify: `cli/cmd/install_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `cli/cmd/install_test.go`:

```go
func TestInstallTarballExtractsDirectory(t *testing.T) {
	// Build a tarball server-side fixture
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "scripts/run.sh"), []byte("echo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tgz, err := skillpkg.Pack(src)
	if err != nil {
		t.Fatal(err)
	}

	srv := newFakeServerWithContent(t, tgz)
	skillsDir := t.TempDir()
	cmd := newRootCmdForTest(t, srv.URL)
	cmd.SetArgs([]string{"install", "demo", "--skills-dir", skillsDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("install: %v", err)
	}
	for _, rel := range []string{"SKILL.md", "scripts/run.sh"} {
		if _, err := os.Stat(filepath.Join(skillsDir, "demo", rel)); err != nil {
			t.Errorf("missing %s: %v", rel, err)
		}
	}
}

func TestInstallRefusesExistingDirWithoutForce(t *testing.T) {
	src := t.TempDir()
	_ = os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("x"), 0o644)
	tgz, _ := skillpkg.Pack(src)
	srv := newFakeServerWithContent(t, tgz)

	skillsDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(skillsDir, "demo"), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmdForTest(t, srv.URL)
	cmd.SetArgs([]string{"install", "demo", "--skills-dir", skillsDir})
	if err := cmd.Execute(); err == nil {
		t.Fatal("want error when destination exists")
	}

	cmd2 := newRootCmdForTest(t, srv.URL)
	cmd2.SetArgs([]string{"install", "demo", "--skills-dir", skillsDir, "--force"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("--force install failed: %v", err)
	}
}

func TestInstallLegacySingleFile(t *testing.T) {
	srv := newFakeServerWithContent(t, []byte("# legacy SKILL.md\n"))
	skillsDir := t.TempDir()
	cmd := newRootCmdForTest(t, srv.URL)
	cmd.SetArgs([]string{"install", "demo", "--skills-dir", skillsDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "demo", "SKILL.md")); err != nil {
		t.Fatalf("missing SKILL.md: %v", err)
	}
}
```

If `newFakeServerWithContent` does not exist, add it next to the existing test fakes in this file. Mirror the existing fake server constructor; have it return the supplied bytes from `GetSkillContent`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cli/cmd/ -run TestInstall`
Expected: FAIL: existing-dir test passes today? It currently overwrites SKILL.md so the new test should fail because the legacy code never errors on existing dir. Tarball extraction tests fail because no extraction logic exists.

- [ ] **Step 3: Update install.go**

```go
// cli/cmd/install.go
package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/nebari-dev/skillsctl/internal/skillpkg"
)

var skillNameRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

func validateSkillName(name string) error {
	if len(name) < 2 || len(name) > 64 {
		return fmt.Errorf("skill name must be between 2 and 64 characters")
	}
	if !skillNameRegexp.MatchString(name) {
		return fmt.Errorf("skill name must be lowercase alphanumeric with hyphens, cannot start or end with a hyphen")
	}
	return nil
}

func addInstallCmd(root *cobra.Command) {
	var (
		digest    string
		skillsDir string
		force     bool
	)

	installCmd := &cobra.Command{
		Use:   "install <name[@version]>",
		Short: "Install a skill from the registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, version := parseNameVersion(args[0])
			if err := validateSkillName(name); err != nil {
				return err
			}

			dir := skillsDir
			if dir == "" {
				dir = viper.GetString("skills_dir")
			}

			client := getClientCtx(cmd.Context())
			content, ver, err := client.GetSkillContent(cmd.Context(), name, version, digest)
			if err != nil {
				return mapInstallError(err, name, version)
			}

			destDir := filepath.Join(dir, name)
			absSkillsDir, _ := filepath.Abs(dir)
			absDest, _ := filepath.Abs(destDir)
			if !strings.HasPrefix(absDest, absSkillsDir+string(filepath.Separator)) {
				return fmt.Errorf("invalid skill name: resolved path escapes skills directory")
			}

			if skillpkg.IsTarball(content) {
				if err := installTarball(content, destDir, force); err != nil {
					return err
				}
			} else {
				if err := installSingleFile(content, destDir); err != nil {
					return err
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Installed %s@%s to %s\n", name, ver.Version, destDir)
			return nil
		},
	}

	installCmd.Flags().StringVar(&digest, "digest", "", "Expected content digest for verification")
	installCmd.Flags().StringVar(&skillsDir, "skills-dir", "", "Override skills directory")
	installCmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing skill directory")

	root.AddCommand(installCmd)
}

func installTarball(content []byte, destDir string, force bool) error {
	if force {
		if err := os.RemoveAll(destDir); err != nil {
			return fmt.Errorf("remove existing %s: %w", destDir, err)
		}
	}
	if err := skillpkg.Extract(content, destDir, skillpkg.DefaultLimits()); err != nil {
		return fmt.Errorf("extract: %w", err)
	}
	return nil
}

func installSingleFile(content []byte, destDir string) error {
	if err := os.MkdirAll(destDir, 0o750); err != nil {
		return fmt.Errorf("create directory %s: %w", destDir, err)
	}
	return atomicWrite(filepath.Join(destDir, "SKILL.md"), content)
}

// parseNameVersion, atomicWrite, mapInstallError unchanged.
```

Keep `parseNameVersion`, `atomicWrite`, and `mapInstallError` as they are today.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cli/cmd/ -race -run TestInstall`
Expected: PASS.

- [ ] **Step 5: Run full test + lint**

Run: `make test && make lint`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cli/cmd/install.go cli/cmd/install_test.go
git commit -m "feat(cli): install extracts tarballs, adds --force"
```

---

## Task 11: Server config wiring for limits

**Files:**
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/cmd/server/main_test.go` (if it covers config)

- [ ] **Step 1: Read current config wiring**

Run: `cat backend/cmd/server/main.go`
Note Viper key prefixes and where `server.New` is called.

- [ ] **Step 2: Add Viper keys with defaults**

Add near other Viper defaults in `main.go`:

```go
viper.SetDefault("limits.max_packed_bytes", 5*1024*1024)
viper.SetDefault("limits.max_total_bytes", 20*1024*1024)
viper.SetDefault("limits.max_files", 100)
viper.SetDefault("limits.max_file_bytes", 1024*1024)
```

Build a `skillpkg.Limits` and pass it in:

```go
limits := skillpkg.Limits{
	MaxPackedBytes: viper.GetInt64("limits.max_packed_bytes"),
	MaxTotalBytes:  viper.GetInt64("limits.max_total_bytes"),
	MaxFiles:       viper.GetInt("limits.max_files"),
	MaxFileBytes:   viper.GetInt64("limits.max_file_bytes"),
}

srv := server.New(repo, validator, authCfg, server.WithLimits(limits))
```

Add the import: `"github.com/nebari-dev/skillsctl/internal/skillpkg"`.

- [ ] **Step 3: Build the binary**

Run: `CGO_ENABLED=0 go build -o /tmp/skillsctl-server ./backend/cmd/server`
Expected: clean build.

- [ ] **Step 4: Run all tests**

Run: `make test && make lint`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/cmd/server/main.go
git commit -m "feat(server): load tarball limits from Viper config"
```

---

## Task 12: E2E test for multi-file publish + install

**Files:**
- Modify: existing e2e harness (location TBD - likely under `cli/cmd` or a top-level `e2e` dir; locate with `grep -rn "e2e" --include='*.go' .`)

- [ ] **Step 1: Locate the e2e test**

Run: `grep -rln "e2e" --include='*.go' .`
Pick the file that already exercises publish + install end-to-end against a running server.

- [ ] **Step 2: Add a multi-file test**

Add a new test that:
1. Creates a temp dir with `SKILL.md` and `scripts/run.sh`.
2. Runs `skillsctl publish --dir <tmp> --name e2e-multi --version 0.1.0 --description ...`.
3. Runs `skillsctl install e2e-multi --skills-dir <other-tmp>`.
4. Asserts both `SKILL.md` and `scripts/run.sh` exist in `<other-tmp>/e2e-multi/` with the expected contents.

Mirror the structure of the nearest existing e2e test for setup/teardown.

- [ ] **Step 3: Run e2e**

Run: `go test ./... -run E2E -race` (or whichever invocation matches the existing e2e tag/build constraints).
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add <e2e files>
git commit -m "test(e2e): publish and install a multi-file skill"
```

---

## Task 13: Documentation

**Files:**
- Modify: `README.md`
- Modify: docs site sources if they exist (`docs/site/content/...`)
- Modify: `CHANGELOG.md` (if present)

- [ ] **Step 1: Update CLI command reference**

In the README and any docs site pages that document `publish` and `install`:
- Replace `--file <path>` with `--dir <path>` for `publish`. Add a short paragraph: "The directory must contain `SKILL.md` at its root. Subdirectories like `scripts/`, `references/`, `assets/` are packaged alongside it."
- Document the new `--force` flag on `install`.
- Document the new `GET /limits` endpoint and the four `limits.*` config keys.

- [ ] **Step 2: Add a "Skill layout" section**

Short section explaining the on-disk layout of a skill directory, with an example tree:

```
my-skill/
  SKILL.md
  scripts/
    run.sh
  references/
    notes.md
```

- [ ] **Step 3: Add a changelog entry**

Add an entry noting:
- BREAKING: `skillsctl publish` now uses `--dir` instead of `--file`. The flag accepts a directory containing `SKILL.md`.
- New: `skillsctl install --force` overwrites an existing skill directory.
- New: `GET /limits` endpoint exposes server-side tarball limits.
- Internal: legacy single-file skill records still install correctly.

- [ ] **Step 4: Commit**

```bash
git add README.md CHANGELOG.md docs/
git commit -m "docs: document --dir publish, --force install, and /limits"
```

---

## Self-Review

- **Spec coverage:**
  - Tarball BLOB storage, no proto/DB changes -> Tasks 7, 11 (no new tables/migrations).
  - Deterministic Pack -> Task 4.
  - Validate (SKILL.md root, no symlinks/links/abs/`..`, file count/size caps) -> Task 3.
  - Extract via temp dir + rename, refuse existing dest -> Task 5.
  - `/limits` endpoint -> Task 6, client method Task 8, CLI fetch Task 9.
  - Server validation on publish -> Task 7.
  - CLI publish `--dir` -> Task 9.
  - CLI install detects gzip magic, legacy fallback, `--force` -> Task 10.
  - Server config keys with defaults -> Task 11.
  - E2E coverage -> Task 12.
  - Docs and changelog -> Task 13.
- **Placeholder scan:** Task 12 references a "TBD" location for the e2e harness; that's an explicit step (locate before edit) rather than a vague instruction. Rest is concrete.
- **Type consistency:** `skillpkg.Limits`, `skillpkg.Pack`, `skillpkg.Validate`, `skillpkg.Extract`, `skillpkg.IsTarball`, `skillpkg.DefaultLimits()` used consistently. `registry.WithLimits` and `server.WithLimits` distinct, both implemented in their respective tasks before being called from elsewhere.
