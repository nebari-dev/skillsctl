package skillpkg

// IsTarball reports whether b begins with the gzip magic bytes. We use this to
// distinguish multi-file skill tarballs from legacy raw SKILL.md content.
func IsTarball(b []byte) bool {
	return len(b) >= 2 && b[0] == 0x1f && b[1] == 0x8b
}
