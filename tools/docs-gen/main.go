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

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	clicmd "github.com/nebari-dev/skillsctl/cli/cmd"
)

func main() {
	out := flag.String("o", "", "output directory (required)")
	flag.Parse()

	if *out == "" {
		fmt.Fprintln(os.Stderr, "docs-gen: -o is required")
		flag.Usage()
		os.Exit(2)
	}
	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "docs-gen: unexpected positional arguments: %v\n", flag.Args())
		os.Exit(2)
	}

	if err := run(clicmd.NewRootCmd(), *out); err != nil {
		log.Fatalf("docs-gen: %v", err)
	}
	fmt.Printf("wrote CLI reference to %s\n", *out)
}

func run(root *cobra.Command, out string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", out, err)
	}
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

	return doc.GenMarkdownTreeCustom(root, out, filePrepender, linkHandler)
}
