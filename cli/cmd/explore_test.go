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

func TestExploreShowVerbose(t *testing.T) {
	content := map[string][]byte{
		"data-pipeline": []byte("# Data Pipeline\nHelps with data pipelines"),
	}
	ts := testutil.NewStubServerWithContent(t, testutil.SeedSkills(), content)

	var buf bytes.Buffer
	root := cmd.NewRootCmd()
	root.SetOut(&buf)
	root.SetArgs([]string{"explore", "show", "data-pipeline", "--verbose", "--api-url", ts.URL})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "--- Content ---") {
		t.Errorf("expected content section header, got:\n%s", output)
	}
	if !strings.Contains(output, "Helps with data pipelines") {
		t.Errorf("expected skill content, got:\n%s", output)
	}
}

func TestExploreShowVerboseNoContent(t *testing.T) {
	// No content map - stub returns CodeNotFound
	ts := testutil.NewStubServer(t, testutil.SeedSkills())

	var buf bytes.Buffer
	var errBuf bytes.Buffer
	root := cmd.NewRootCmd()
	root.SetOut(&buf)
	root.SetErr(&errBuf)
	root.SetArgs([]string{"explore", "show", "data-pipeline", "--verbose", "--api-url", ts.URL})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still show metadata
	if !strings.Contains(buf.String(), "data-pipeline") {
		t.Errorf("expected skill metadata, got:\n%s", buf.String())
	}
	// Warning should appear on stderr
	if !strings.Contains(errBuf.String(), "Warning") {
		t.Errorf("expected warning on stderr, got:\n%s", errBuf.String())
	}
}
