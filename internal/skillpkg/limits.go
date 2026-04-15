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
