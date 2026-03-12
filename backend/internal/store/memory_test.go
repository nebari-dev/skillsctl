package store_test

import (
	"context"
	"testing"

	skillctlv1 "github.com/nebari-dev/skillctl/gen/go/skillctl/v1"

	"github.com/nebari-dev/skillctl/backend/internal/store"
)

func testSkills() []*skillctlv1.Skill {
	return []*skillctlv1.Skill{
		{
			Name:          "data-pipeline",
			Description:   "Data pipeline utilities",
			Owner:         "data-eng",
			Tags:          []string{"data", "spark", "go"},
			LatestVersion: "1.3.0",
			InstallCount:  47,
			Source:        skillctlv1.SkillSource_SKILL_SOURCE_INTERNAL,
		},
		{
			Name:          "code-review",
			Description:   "Code review helpers",
			Owner:         "platform",
			Tags:          []string{"review", "quality"},
			LatestVersion: "0.9.1",
			InstallCount:  23,
			Source:        skillctlv1.SkillSource_SKILL_SOURCE_INTERNAL,
		},
	}
}

func TestMemoryStore_ListSkills(t *testing.T) {
	tests := []struct {
		name         string
		tags         []string
		sourceFilter skillctlv1.SkillSource
		wantCount    int
	}{
		{
			name:      "list all skills",
			wantCount: 2,
		},
		{
			name:      "filter by matching tag",
			tags:      []string{"go"},
			wantCount: 1,
		},
		{
			name:      "filter by non-matching tag",
			tags:      []string{"nonexistent"},
			wantCount: 0,
		},
		{
			name:         "filter by source internal",
			sourceFilter: skillctlv1.SkillSource_SKILL_SOURCE_INTERNAL,
			wantCount:    2,
		},
		{
			name:         "filter by source federated returns none",
			sourceFilter: skillctlv1.SkillSource_SKILL_SOURCE_FEDERATED,
			wantCount:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := store.NewMemory(testSkills())
			skills, _, err := s.ListSkills(context.Background(), tt.tags, tt.sourceFilter, 20, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(skills) != tt.wantCount {
				t.Errorf("expected %d skills, got %d", tt.wantCount, len(skills))
			}
		})
	}
}

func TestMemoryStore_ListSkills_Empty(t *testing.T) {
	s := store.NewMemory(nil)
	skills, _, err := s.ListSkills(context.Background(), nil, skillctlv1.SkillSource_SKILL_SOURCE_UNSPECIFIED, 20, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestMemoryStore_GetSkill(t *testing.T) {
	tests := []struct {
		name      string
		skillName string
		wantErr   bool
	}{
		{
			name:      "existing skill",
			skillName: "data-pipeline",
		},
		{
			name:      "nonexistent skill",
			skillName: "does-not-exist",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := store.NewMemory(testSkills())
			skill, _, err := s.GetSkill(context.Background(), tt.skillName)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if skill.Name != tt.skillName {
				t.Errorf("expected name %q, got %q", tt.skillName, skill.Name)
			}
		})
	}
}
