//go:build e2e

package testutil

import (
	"bytes"
	"errors"
	"os/exec"
	"testing"
)

// Result holds the output and exit code from a CLI invocation.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// CLIRunner wraps os/exec calls to the skillsctl binary.
type CLIRunner struct {
	BinaryPath string
	ServerURL  string
	SkillsDir  string
}

// NewCLIRunner creates a CLIRunner with a per-test temp skills directory.
func NewCLIRunner(t *testing.T, binaryPath, serverURL string) *CLIRunner {
	t.Helper()
	return &CLIRunner{
		BinaryPath: binaryPath,
		ServerURL:  serverURL,
		SkillsDir:  t.TempDir(),
	}
}

// Run executes the CLI binary with the given args, automatically injecting
// --api-url flag and SKILLCTL_SKILLS_DIR env var. Returns stdout, stderr,
// and exit code.
func (r *CLIRunner) Run(args ...string) *Result {
	fullArgs := append(args, "--api-url", r.ServerURL)
	cmd := exec.Command(r.BinaryPath, fullArgs...)
	cmd.Env = append(cmd.Environ(), "SKILLCTL_SKILLS_DIR="+r.SkillsDir)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return &Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}
