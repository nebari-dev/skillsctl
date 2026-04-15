package skillpkg

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
)

// ReadSkillMd returns the contents of SKILL.md from a gzip-compressed tar
// archive. It is intended for display (e.g., `explore show --verbose`) - not
// for extraction. Callers that need the full archive should use Extract.
func ReadSkillMd(tarball []byte) ([]byte, error) {
	gr, err := gzip.NewReader(bytes.NewReader(tarball))
	if err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}
	defer func() { _ = gr.Close() }()
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, errors.New("SKILL.md not found in tarball")
		}
		if err != nil {
			return nil, fmt.Errorf("tar: %w", err)
		}
		if hdr.Name != "SKILL.md" || hdr.Typeflag != tar.TypeReg {
			continue
		}
		buf := bytes.NewBuffer(make([]byte, 0, hdr.Size))
		if _, err := io.Copy(buf, io.LimitReader(tr, hdr.Size+1)); err != nil {
			return nil, fmt.Errorf("read SKILL.md: %w", err)
		}
		return buf.Bytes(), nil
	}
}
