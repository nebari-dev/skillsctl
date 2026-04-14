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

// IsTarball reports whether b begins with the gzip magic bytes. We use this to
// distinguish multi-file skill tarballs from legacy raw SKILL.md content.
func IsTarball(b []byte) bool {
	return len(b) >= 2 && b[0] == 0x1f && b[1] == 0x8b
}

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
	defer func() { _ = gr.Close() }()

	tr := tar.NewReader(gr)
	var (
		fileCount  int
		totalBytes int64
		hasSkillMd bool
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
			return fmt.Errorf("file %q size %d exceeds per-file limit %d (file size)", hdr.Name, hdr.Size, limits.MaxFileBytes)
		}
		totalBytes += hdr.Size
		if limits.MaxTotalBytes > 0 && totalBytes > limits.MaxTotalBytes {
			return fmt.Errorf("total uncompressed size exceeds limit %d", limits.MaxTotalBytes)
		}
		lr := io.LimitReader(tr, hdr.Size+1)
		if _, err := io.Copy(io.Discard, lr); err != nil {
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
	default:
		return fmt.Errorf("unsupported tar entry type %c for %q", hdr.Typeflag, hdr.Name)
	}
	name := hdr.Name
	if strings.HasPrefix(name, "/") {
		return fmt.Errorf("absolute path %q not allowed", name)
	}
	clean := path.Clean(name)
	if clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains("/"+clean+"/", "/../") {
		return fmt.Errorf("path %q contains parent traversal", name)
	}
	return nil
}
