package cmd_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nebari-dev/skillctl/cli/cmd"
	"github.com/nebari-dev/skillctl/cli/internal/testutil"
)

func TestExploreCommand(t *testing.T) {
	ts := testutil.NewStubServer(t, testutil.SeedSkills())

	var buf bytes.Buffer
	root := cmd.NewRootCmd()
	root.SetOut(&buf)
	root.SetArgs([]string{"explore", "--api-url", ts.URL})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "data-pipeline") {
		t.Errorf("expected output to contain 'data-pipeline', got:\n%s", output)
	}
	if !strings.Contains(output, "code-review") {
		t.Errorf("expected output to contain 'code-review', got:\n%s", output)
	}
	if !strings.Contains(output, "SOURCE") {
		t.Errorf("expected table header, got:\n%s", output)
	}
}

func TestExploreShowCommand(t *testing.T) {
	ts := testutil.NewStubServer(t, testutil.SeedSkills())

	var buf bytes.Buffer
	root := cmd.NewRootCmd()
	root.SetOut(&buf)
	root.SetArgs([]string{"explore", "show", "data-pipeline", "--api-url", ts.URL})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "data-pipeline") {
		t.Errorf("expected 'data-pipeline' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Description:") {
		t.Errorf("expected 'Description:' in output, got:\n%s", output)
	}
}
