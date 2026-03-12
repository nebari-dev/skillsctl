package store_test

import (
	"context"
	"database/sql"
	"reflect"
	"testing"

	_ "modernc.org/sqlite"

	skillctlv1 "github.com/nebari-dev/skillctl/gen/go/skillctl/v1"

	"github.com/nebari-dev/skillctl/backend/internal/store"
	"github.com/nebari-dev/skillctl/backend/internal/store/migrations"
)

// openTestDB creates an in-memory SQLite database with pragmas and migrations applied.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := migrations.Run(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	return db
}

// seedSkills inserts test skills into the database.
func seedSkills(t *testing.T, db *sql.DB) {
	t.Helper()
	skills := []struct {
		name, desc, owner, tags, version string
		installs                         int
		source                           int
	}{
		{"data-pipeline", "Data pipeline utilities", "data-eng", `["data","spark","go"]`, "1.3.0", 47, 1},
		{"code-review", "Code review helpers", "platform", `["review","quality"]`, "0.9.1", 23, 1},
	}
	for _, s := range skills {
		_, err := db.Exec(
			`INSERT INTO skills (name, description, owner, tags, latest_version, install_count, source) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			s.name, s.desc, s.owner, s.tags, s.version, s.installs, s.source,
		)
		if err != nil {
			t.Fatalf("seed skill %s: %v", s.name, err)
		}
	}
}

func TestSQLiteStore_ListSkills(t *testing.T) {
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
		{
			name:      "filter by multiple tags matches if any match",
			tags:      []string{"go", "nonexistent"},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openTestDB(t)
			seedSkills(t, db)
			s := store.NewSQLite(db)
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

func TestSQLiteStore_ListSkills_Empty(t *testing.T) {
	db := openTestDB(t)
	s := store.NewSQLite(db)
	skills, _, err := s.ListSkills(context.Background(), nil, skillctlv1.SkillSource_SKILL_SOURCE_UNSPECIFIED, 20, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestSQLiteStore_GetSkill(t *testing.T) {
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
			db := openTestDB(t)
			seedSkills(t, db)
			s := store.NewSQLite(db)
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

func TestSQLiteStore_GetSkill_TagsRoundTrip(t *testing.T) {
	db := openTestDB(t)
	seedSkills(t, db)
	s := store.NewSQLite(db)

	skill, _, err := s.GetSkill(context.Background(), "data-pipeline")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantTags := []string{"data", "spark", "go"}
	if !reflect.DeepEqual(skill.Tags, wantTags) {
		t.Errorf("expected tags %v, got %v", wantTags, skill.Tags)
	}
}

func TestSQLiteStore_GetSkill_WithVersions(t *testing.T) {
	db := openTestDB(t)
	seedSkills(t, db)
	_, err := db.Exec(
		`INSERT INTO skill_versions (skill_name, version, oci_ref, digest, published_by, changelog) VALUES (?, ?, ?, ?, ?, ?)`,
		"data-pipeline", "1.3.0", "ghcr.io/org/skills/data-pipeline:1.3.0", "sha256:abc123", "user@example.com", "Initial release",
	)
	if err != nil {
		t.Fatalf("insert version: %v", err)
	}

	s := store.NewSQLite(db)
	skill, versions, err := s.GetSkill(context.Background(), "data-pipeline")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skill.Name != "data-pipeline" {
		t.Errorf("expected name data-pipeline, got %q", skill.Name)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}
	if versions[0].Version != "1.3.0" {
		t.Errorf("expected version 1.3.0, got %q", versions[0].Version)
	}
	if versions[0].OciRef != "ghcr.io/org/skills/data-pipeline:1.3.0" {
		t.Errorf("expected oci_ref, got %q", versions[0].OciRef)
	}
}
