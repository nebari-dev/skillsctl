// Package skills embeds the default skill files for use by the server's seed package.
package skills

import "embed"

//go:embed *.md
var FS embed.FS
