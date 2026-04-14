package skillpkg

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"
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
		{"dotdot", []tarEntry{{name: "../SKILL.md", mode: 0o644, body: []byte("x")}}, "parent traversal"},
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
