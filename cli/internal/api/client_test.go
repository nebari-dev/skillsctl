package api_test

import (
	"context"
	"testing"

	skillctlv1 "github.com/nebari-dev/skillctl/gen/go/skillctl/v1"

	"github.com/nebari-dev/skillctl/cli/internal/api"
	"github.com/nebari-dev/skillctl/cli/internal/testutil"
)

func TestClient_ListSkills(t *testing.T) {
	ts := testutil.NewStubServer(t, testutil.SeedSkills())
	client := api.NewClient(ts.URL)

	skills, err := client.ListSkills(context.Background(), nil, skillctlv1.SkillSource_SKILL_SOURCE_UNSPECIFIED)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(skills))
	}
}

func TestClient_GetSkill(t *testing.T) {
	ts := testutil.NewStubServer(t, testutil.SeedSkills())
	client := api.NewClient(ts.URL)

	tests := []struct {
		name    string
		skill   string
		wantErr bool
	}{
		{"existing", "data-pipeline", false},
		{"not found", "nope", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skill, _, err := client.GetSkill(context.Background(), tt.skill)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if skill.Name != tt.skill {
				t.Errorf("expected %q, got %q", tt.skill, skill.Name)
			}
		})
	}
}
