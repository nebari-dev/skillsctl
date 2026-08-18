package seed_test

import (
	"context"
	"testing"

	"github.com/nebari-dev/skillsctl/backend/internal/seed"
	"github.com/nebari-dev/skillsctl/backend/internal/store"
	skillsctlv1 "github.com/nebari-dev/skillsctl/gen/go/skillsctl/v1"
)

func TestRun(t *testing.T) {
	skills := []seed.Skill{{
		Name:        "skillsctl-usage",
		Description: "guide",
		Tags:        []string{"cli"},
		Content:     []byte("# guide"),
	}}

	tests := []struct {
		name        string
		existing    []*skillsctlv1.Skill
		version     string
		preSeed     bool
		wantVersion string
		wantOutcome seed.Outcome
	}{
		{
			name:        "fresh registry publishes the version",
			version:     "0.2.3",
			wantVersion: "0.2.3",
			wantOutcome: seed.OutcomeSeeded,
		},
		{
			name:        "empty version falls back to 0.0.0",
			version:     "",
			wantVersion: "0.0.0",
			wantOutcome: seed.OutcomeSeeded,
		},
		{
			name:        "rerun at the same version reports already-present",
			version:     "0.2.3",
			preSeed:     true,
			wantVersion: "0.2.3",
			wantOutcome: seed.OutcomeAlreadyPresent,
		},
		{
			name:        "skill owned by another publisher reports not-owner",
			existing:    []*skillsctlv1.Skill{{Name: "skillsctl-usage", Owner: "someone-else", Description: "theirs"}},
			version:     "0.2.3",
			wantVersion: "0.2.3",
			wantOutcome: seed.OutcomeNotOwner,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := store.NewMemory(tt.existing)
			if tt.preSeed {
				if _, err := seed.Run(context.Background(), repo, tt.version, skills); err != nil {
					t.Fatalf("pre-seed: %v", err)
				}
			}

			got, err := seed.Run(context.Background(), repo, tt.version, skills)
			if err != nil {
				t.Fatalf("Run() unexpected error: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("Run() returned %d results, want 1", len(got))
			}
			if got[0].Name != "skillsctl-usage" {
				t.Errorf("Name = %q, want skillsctl-usage", got[0].Name)
			}
			if got[0].Version != tt.wantVersion {
				t.Errorf("Version = %q, want %q", got[0].Version, tt.wantVersion)
			}
			if got[0].Outcome != tt.wantOutcome {
				t.Errorf("Outcome = %q, want %q", got[0].Outcome, tt.wantOutcome)
			}
		})
	}
}
