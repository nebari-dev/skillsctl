package store_test

import (
	"context"
	"errors"
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
				if !errors.Is(err, store.ErrNotFound) {
					t.Errorf("expected ErrNotFound, got %v", err)
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

func TestMemoryStore_CreateSkillVersion(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, m *store.Memory)
		skill   *skillctlv1.Skill
		version *skillctlv1.SkillVersion
		content []byte
		wantErr error
	}{
		{
			name: "first publish creates skill",
			skill: &skillctlv1.Skill{
				Name:        "new-skill",
				Description: "A brand new skill",
				Owner:       "alice",
				Tags:        []string{"test"},
			},
			version: &skillctlv1.SkillVersion{
				Version:   "1.0.0",
				Changelog: "initial release",
				Digest:    "sha256:abc123",
			},
			content: []byte("skill content v1"),
		},
		{
			name: "second version by same owner succeeds",
			setup: func(t *testing.T, m *store.Memory) {
				t.Helper()
				err := m.CreateSkillVersion(context.Background(), &skillctlv1.Skill{
					Name:  "existing-skill",
					Owner: "alice",
				}, &skillctlv1.SkillVersion{
					Version: "1.0.0",
					Digest:  "sha256:abc",
				}, []byte("v1 content"))
				if err != nil {
					t.Fatalf("setup: %v", err)
				}
			},
			skill: &skillctlv1.Skill{
				Name:  "existing-skill",
				Owner: "alice",
			},
			version: &skillctlv1.SkillVersion{
				Version: "2.0.0",
				Digest:  "sha256:def",
			},
			content: []byte("v2 content"),
		},
		{
			name: "different owner rejected",
			setup: func(t *testing.T, m *store.Memory) {
				t.Helper()
				err := m.CreateSkillVersion(context.Background(), &skillctlv1.Skill{
					Name:  "owned-skill",
					Owner: "alice",
				}, &skillctlv1.SkillVersion{
					Version: "1.0.0",
					Digest:  "sha256:abc",
				}, []byte("v1 content"))
				if err != nil {
					t.Fatalf("setup: %v", err)
				}
			},
			skill: &skillctlv1.Skill{
				Name:  "owned-skill",
				Owner: "bob",
			},
			version: &skillctlv1.SkillVersion{
				Version: "2.0.0",
				Digest:  "sha256:def",
			},
			content: []byte("bob content"),
			wantErr: store.ErrPermissionDenied,
		},
		{
			name: "duplicate version rejected",
			setup: func(t *testing.T, m *store.Memory) {
				t.Helper()
				err := m.CreateSkillVersion(context.Background(), &skillctlv1.Skill{
					Name:  "dup-skill",
					Owner: "alice",
				}, &skillctlv1.SkillVersion{
					Version: "1.0.0",
					Digest:  "sha256:abc",
				}, []byte("v1 content"))
				if err != nil {
					t.Fatalf("setup: %v", err)
				}
			},
			skill: &skillctlv1.Skill{
				Name:  "dup-skill",
				Owner: "alice",
			},
			version: &skillctlv1.SkillVersion{
				Version: "1.0.0",
				Digest:  "sha256:abc",
			},
			content: []byte("v1 content again"),
			wantErr: store.ErrAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := store.NewMemory(nil)
			if tt.setup != nil {
				tt.setup(t, m)
			}

			err := m.CreateSkillVersion(context.Background(), tt.skill, tt.version, tt.content)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify the skill is retrievable after creation.
			skill, _, getErr := m.GetSkill(context.Background(), tt.skill.Name)
			if getErr != nil {
				t.Fatalf("GetSkill after create: %v", getErr)
			}
			if skill.Source != skillctlv1.SkillSource_SKILL_SOURCE_INTERNAL {
				t.Errorf("expected INTERNAL source, got %v", skill.Source)
			}
		})
	}
}

func TestMemoryStore_GetSkillContent(t *testing.T) {
	setupStore := func(t *testing.T) *store.Memory {
		t.Helper()
		m := store.NewMemory(nil)
		err := m.CreateSkillVersion(context.Background(), &skillctlv1.Skill{
			Name:  "my-skill",
			Owner: "alice",
		}, &skillctlv1.SkillVersion{
			Version: "1.0.0",
			Digest:  "sha256:aaa",
		}, []byte("content v1"))
		if err != nil {
			t.Fatalf("setup v1: %v", err)
		}
		err = m.CreateSkillVersion(context.Background(), &skillctlv1.Skill{
			Name:  "my-skill",
			Owner: "alice",
		}, &skillctlv1.SkillVersion{
			Version: "2.0.0",
			Digest:  "sha256:bbb",
		}, []byte("content v2"))
		if err != nil {
			t.Fatalf("setup v2: %v", err)
		}
		return m
	}

	tests := []struct {
		name        string
		skillName   string
		version     string
		digest      string
		wantContent string
		wantVersion string
		wantErr     error
	}{
		{
			name:        "get by version",
			skillName:   "my-skill",
			version:     "1.0.0",
			wantContent: "content v1",
			wantVersion: "1.0.0",
		},
		{
			name:        "get latest when version empty",
			skillName:   "my-skill",
			version:     "",
			wantContent: "content v2",
			wantVersion: "2.0.0",
		},
		{
			name:      "nonexistent skill",
			skillName: "no-such-skill",
			version:   "1.0.0",
			wantErr:   store.ErrNotFound,
		},
		{
			name:      "nonexistent version",
			skillName: "my-skill",
			version:   "9.9.9",
			wantErr:   store.ErrNotFound,
		},
		{
			name:        "matching digest passes",
			skillName:   "my-skill",
			version:     "1.0.0",
			digest:      "sha256:aaa",
			wantContent: "content v1",
			wantVersion: "1.0.0",
		},
		{
			name:      "mismatched digest",
			skillName: "my-skill",
			version:   "1.0.0",
			digest:    "sha256:wrong",
			wantErr:   store.ErrDigestMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := setupStore(t)

			content, ver, err := m.GetSkillContent(context.Background(), tt.skillName, tt.version, tt.digest)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(content) != tt.wantContent {
				t.Errorf("expected content %q, got %q", tt.wantContent, string(content))
			}
			if ver.Version != tt.wantVersion {
				t.Errorf("expected version %q, got %q", tt.wantVersion, ver.Version)
			}
		})
	}
}

func TestMemoryStore_CreateSkillVersion_SemverLatest(t *testing.T) {
	m := store.NewMemory(nil)
	ctx := context.Background()

	// Publish 2.0.0 first.
	err := m.CreateSkillVersion(ctx, &skillctlv1.Skill{
		Name:  "semver-skill",
		Owner: "alice",
	}, &skillctlv1.SkillVersion{
		Version: "2.0.0",
		Digest:  "sha256:v2",
	}, []byte("v2"))
	if err != nil {
		t.Fatalf("publish 2.0.0: %v", err)
	}

	// Publish 1.0.0 after - latest should stay at 2.0.0.
	err = m.CreateSkillVersion(ctx, &skillctlv1.Skill{
		Name:  "semver-skill",
		Owner: "alice",
	}, &skillctlv1.SkillVersion{
		Version: "1.0.0",
		Digest:  "sha256:v1",
	}, []byte("v1"))
	if err != nil {
		t.Fatalf("publish 1.0.0: %v", err)
	}

	skill, _, err := m.GetSkill(ctx, "semver-skill")
	if err != nil {
		t.Fatalf("GetSkill: %v", err)
	}
	if skill.LatestVersion != "2.0.0" {
		t.Errorf("expected latest_version 2.0.0, got %s", skill.LatestVersion)
	}
}
