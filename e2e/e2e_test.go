//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"testing"

	"github.com/nebari-dev/skillsctl/e2e/testutil"
)

var (
	cliPath   string
	serverURL string
)

func TestMain(m *testing.M) {
	cliPath = os.Getenv("SKILLCTL_CLI_PATH")
	serverURL = os.Getenv("SKILLCTL_SERVER_URL")

	if cliPath == "" || serverURL == "" {
		fmt.Fprintln(os.Stderr, "skipping e2e: set SKILLCTL_CLI_PATH and SKILLCTL_SERVER_URL")
		os.Exit(0)
	}

	os.Exit(m.Run())
}

// newRunner creates a CLIRunner with per-test isolation.
func newRunner(t *testing.T) *testutil.CLIRunner {
	t.Helper()
	return testutil.NewCLIRunner(t, cliPath, serverURL)
}

// skillName returns a unique skill name derived from the test name,
// sanitized to match the validation pattern ^[a-z0-9][a-z0-9-]*[a-z0-9]$.
func skillName(t *testing.T, suffix string) string {
	t.Helper()
	name := "e2e-" + suffix
	if len(name) < 2 {
		name = "e2e-test"
	}
	return name
}
