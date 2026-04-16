// Command docs-gen renders the skillsctl CLI reference as Hugo-compatible
// Markdown by walking the Cobra command tree.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra/doc"

	clicmd "github.com/nebari-dev/skillsctl/cli/cmd"
)

func main() {
	out := flag.String("o", "docs/site/content/cli/reference", "output directory")
	flag.Parse()

	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", *out, err)
	}

	root := clicmd.NewRootCmd()
	root.DisableAutoGenTag = true

	filePrepender := func(filename string) string {
		base := strings.TrimSuffix(filepath.Base(filename), ".md")
		title := strings.ReplaceAll(base, "_", " ")
		return fmt.Sprintf("---\ntitle: %q\n---\n\n", title)
	}

	linkHandler := func(name string) string {
		base := strings.TrimSuffix(name, ".md")
		return fmt.Sprintf("{{< relref \"/cli/reference/%s\" >}}", base)
	}

	if err := doc.GenMarkdownTreeCustom(root, *out, filePrepender, linkHandler); err != nil {
		log.Fatalf("generate: %v", err)
	}

	fmt.Printf("wrote CLI reference to %s\n", *out)
}
