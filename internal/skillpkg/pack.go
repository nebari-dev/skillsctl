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
	"time"
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
	gw.ModTime = epochUTC
	gw.Name = ""
	tw := tar.NewWriter(gw)

	for _, e := range entries {
		hdr := &tar.Header{
			Name:    e.rel,
			ModTime: epochUTC,
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
		hdr.PAXRecords = nil
		hdr.AccessTime = epochUTC
		hdr.ChangeTime = epochUTC
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

var epochUTC = time.Unix(0, 0).UTC()
