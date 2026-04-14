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
