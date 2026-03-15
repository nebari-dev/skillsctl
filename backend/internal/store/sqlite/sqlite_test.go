package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"

	_ "modernc.org/sqlite"

	skillctlv1 "github.com/nebari-dev/skillctl/gen/go/skillctl/v1"

	"github.com/nebari-dev/skillctl/backend/internal/store"
	sqlitestore "github.com/nebari-dev/skillctl/backend/internal/store/sqlite"
	"github.com/nebari-dev/skillctl/backend/internal/store/sqlite/migrations"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sqlitestore.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := migrations.Run(context.Background(), db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	return db
}

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

func TestRepository_ListSkills(t *testing.T) {
	tests := []struct {
		name         string
		seed         bool
		tags         []string
		sourceFilter skillctlv1.SkillSource
		wantCount    int
	}{
		{
			name:      "empty database",
			seed:      false,
			wantCount: 0,
		},
		{
			name:      "list all skills",
			seed:      true,
			wantCount: 2,
		},
		{
			name:      "filter by matching tag",
			seed:      true,
			tags:      []string{"go"},
			wantCount: 1,
		},
		{
			name:      "filter by non-matching tag",
			seed:      true,
			tags:      []string{"nonexistent"},
			wantCount: 0,
		},
		{
			name:         "filter by source internal",
			seed:         true,
			sourceFilter: skillctlv1.SkillSource_SKILL_SOURCE_INTERNAL,
			wantCount:    2,
		},
		{
			name:         "filter by source federated returns none",
			seed:         true,
			sourceFilter: skillctlv1.SkillSource_SKILL_SOURCE_FEDERATED,
			wantCount:    0,
		},
		{
			name:      "filter by multiple tags matches if any match",
			seed:      true,
			tags:      []string{"go", "nonexistent"},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openTestDB(t)
			if tt.seed {
				seedSkills(t, db)
			}
			repo := sqlitestore.New(db)
			skills, _, err := repo.ListSkills(context.Background(), tt.tags, tt.sourceFilter, 20, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(skills) != tt.wantCount {
				t.Errorf("expected %d skills, got %d", tt.wantCount, len(skills))
			}
		})
	}
}

func TestRepository_ListSkills_RejectsPageToken(t *testing.T) {
	db := openTestDB(t)
	repo := sqlitestore.New(db)
	_, _, err := repo.ListSkills(context.Background(), nil, skillctlv1.SkillSource_SKILL_SOURCE_UNSPECIFIED, 20, "some-token")
	if !errors.Is(err, store.ErrPaginationNotSupported) {
		t.Errorf("expected ErrPaginationNotSupported, got %v", err)
	}
}

func TestRepository_GetSkill(t *testing.T) {
	tests := []struct {
		name      string
		skillName string
		wantErr   error
	}{
		{
			name:      "existing skill",
			skillName: "data-pipeline",
		},
		{
			name:      "nonexistent skill",
			skillName: "does-not-exist",
			wantErr:   store.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openTestDB(t)
			seedSkills(t, db)
			repo := sqlitestore.New(db)
			skill, _, err := repo.GetSkill(context.Background(), tt.skillName)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
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

func TestRepository_GetSkill_TagsRoundTrip(t *testing.T) {
	db := openTestDB(t)
	seedSkills(t, db)
	repo := sqlitestore.New(db)

	skill, _, err := repo.GetSkill(context.Background(), "data-pipeline")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantTags := []string{"data", "spark", "go"}
	if !reflect.DeepEqual(skill.Tags, wantTags) {
		t.Errorf("expected tags %v, got %v", wantTags, skill.Tags)
	}
}

func TestRepository_GetSkill_WithVersions(t *testing.T) {
	db := openTestDB(t)
	seedSkills(t, db)
	_, err := db.Exec(
		`INSERT INTO skill_versions (skill_name, version, oci_ref, digest, published_by, changelog) VALUES (?, ?, ?, ?, ?, ?)`,
		"data-pipeline", "1.3.0", "ghcr.io/org/skills/data-pipeline:1.3.0", "sha256:abc123", "user@example.com", "Initial release",
	)
	if err != nil {
		t.Fatalf("insert version: %v", err)
	}

	repo := sqlitestore.New(db)
	skill, versions, err := repo.GetSkill(context.Background(), "data-pipeline")
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

func TestRepository_CreateSkillVersion(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*testing.T, *sql.DB)
		skill   *skillctlv1.Skill
		version *skillctlv1.SkillVersion
		content []byte
		wantErr error
	}{
		{
			name:  "first publish creates skill and version",
			setup: func(t *testing.T, db *sql.DB) {},
			skill: &skillctlv1.Skill{
				Name:        "new-skill",
				Description: "A new skill",
				Owner:       "user-123",
				Tags:        []string{"go", "testing"},
			},
			version: &skillctlv1.SkillVersion{
				Version:     "1.0.0",
				PublishedBy: "user@example.com",
				Digest:      "sha256:abc123",
				SizeBytes:   100,
				Changelog:   "Initial release",
			},
			content: []byte("# My Skill\nDoes stuff"),
		},
		{
			name: "second version by same owner succeeds",
			setup: func(t *testing.T, db *sql.DB) {
				t.Helper()
				repo := sqlitestore.New(db)
				err := repo.CreateSkillVersion(context.Background(),
					&skillctlv1.Skill{Name: "existing", Description: "test", Owner: "user-123", Tags: []string{}},
					&skillctlv1.SkillVersion{Version: "1.0.0", PublishedBy: "u@ex.com", Digest: "sha256:a", SizeBytes: 5},
					[]byte("v1"),
				)
				if err != nil {
					t.Fatalf("setup: %v", err)
				}
			},
			skill: &skillctlv1.Skill{
				Name:        "existing",
				Description: "updated desc",
				Owner:       "user-123",
				Tags:        []string{"updated"},
			},
			version: &skillctlv1.SkillVersion{
				Version:     "2.0.0",
				PublishedBy: "u@ex.com",
				Digest:      "sha256:b",
				SizeBytes:   5,
			},
			content: []byte("v2"),
		},
		{
			name: "different owner rejected",
			setup: func(t *testing.T, db *sql.DB) {
				t.Helper()
				repo := sqlitestore.New(db)
				if err := repo.CreateSkillVersion(context.Background(),
					&skillctlv1.Skill{Name: "owned", Description: "test", Owner: "user-123", Tags: []string{}},
					&skillctlv1.SkillVersion{Version: "1.0.0", PublishedBy: "u@ex.com", Digest: "sha256:a", SizeBytes: 5},
					[]byte("v1"),
				); err != nil {
					t.Fatalf("setup: %v", err)
				}
			},
			skill:   &skillctlv1.Skill{Name: "owned", Owner: "other-user"},
			version: &skillctlv1.SkillVersion{Version: "2.0.0", Digest: "sha256:b", SizeBytes: 5},
			content: []byte("v2"),
			wantErr: store.ErrPermissionDenied,
		},
		{
			name: "duplicate version rejected",
			setup: func(t *testing.T, db *sql.DB) {
				t.Helper()
				repo := sqlitestore.New(db)
				if err := repo.CreateSkillVersion(context.Background(),
					&skillctlv1.Skill{Name: "dup", Description: "test", Owner: "user-123", Tags: []string{}},
					&skillctlv1.SkillVersion{Version: "1.0.0", PublishedBy: "u@ex.com", Digest: "sha256:a", SizeBytes: 5},
					[]byte("v1"),
				); err != nil {
					t.Fatalf("setup: %v", err)
				}
			},
			skill:   &skillctlv1.Skill{Name: "dup", Owner: "user-123"},
			version: &skillctlv1.SkillVersion{Version: "1.0.0", Digest: "sha256:b", SizeBytes: 5},
			content: []byte("v1-again"),
			wantErr: store.ErrAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openTestDB(t)
			tt.setup(t, db)
			repo := sqlitestore.New(db)
			err := repo.CreateSkillVersion(context.Background(), tt.skill, tt.version, tt.content)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify the skill was created/updated
			skill, versions, err := repo.GetSkill(context.Background(), tt.skill.Name)
			if err != nil {
				t.Fatalf("GetSkill after create: %v", err)
			}
			if skill.Name != tt.skill.Name {
				t.Errorf("expected name %q, got %q", tt.skill.Name, skill.Name)
			}
			// Verify at least one version exists
			found := false
			for _, v := range versions {
				if v.Version == tt.version.Version {
					found = true
					if v.Digest != tt.version.Digest {
						t.Errorf("expected digest %q, got %q", tt.version.Digest, v.Digest)
					}
				}
			}
			if !found {
				t.Errorf("version %s not found after create", tt.version.Version)
			}
		})
	}
}

func TestRepository_CreateSkillVersion_SemverLatest(t *testing.T) {
	db := openTestDB(t)
	repo := sqlitestore.New(db)

	// Publish 2.0.0 first
	err := repo.CreateSkillVersion(context.Background(),
		&skillctlv1.Skill{Name: "sv", Description: "test", Owner: "u", Tags: []string{}},
		&skillctlv1.SkillVersion{Version: "2.0.0", PublishedBy: "u@ex.com", Digest: "sha256:a", SizeBytes: 2},
		[]byte("v2"),
	)
	if err != nil {
		t.Fatalf("publish 2.0.0: %v", err)
	}

	// Publish 1.0.0 after - latest should stay at 2.0.0
	err = repo.CreateSkillVersion(context.Background(),
		&skillctlv1.Skill{Name: "sv", Owner: "u"},
		&skillctlv1.SkillVersion{Version: "1.0.0", PublishedBy: "u@ex.com", Digest: "sha256:b", SizeBytes: 2},
		[]byte("v1"),
	)
	if err != nil {
		t.Fatalf("publish 1.0.0: %v", err)
	}

	skill, _, err := repo.GetSkill(context.Background(), "sv")
	if err != nil {
		t.Fatalf("get skill: %v", err)
	}
	if skill.LatestVersion != "2.0.0" {
		t.Errorf("expected latest_version 2.0.0, got %s", skill.LatestVersion)
	}
}

func TestRepository_CreateSkillVersion_UpdatedAtAlwaysSet(t *testing.T) {
	db := openTestDB(t)
	repo := sqlitestore.New(db)

	// Publish 2.0.0
	err := repo.CreateSkillVersion(context.Background(),
		&skillctlv1.Skill{Name: "ts", Description: "test", Owner: "u", Tags: []string{}},
		&skillctlv1.SkillVersion{Version: "2.0.0", PublishedBy: "u@ex.com", Digest: "sha256:a", SizeBytes: 2},
		[]byte("v2"),
	)
	if err != nil {
		t.Fatalf("publish 2.0.0: %v", err)
	}

	// Read updated_at after first version
	var updatedAt1 string
	err = db.QueryRow("SELECT updated_at FROM skills WHERE name = ?", "ts").Scan(&updatedAt1)
	if err != nil {
		t.Fatalf("query updated_at: %v", err)
	}

	// Publish 1.0.0 (older version - should NOT update latest_version but SHOULD update updated_at)
	err = repo.CreateSkillVersion(context.Background(),
		&skillctlv1.Skill{Name: "ts", Owner: "u"},
		&skillctlv1.SkillVersion{Version: "1.0.0", PublishedBy: "u@ex.com", Digest: "sha256:b", SizeBytes: 2},
		[]byte("v1"),
	)
	if err != nil {
		t.Fatalf("publish 1.0.0: %v", err)
	}

	var updatedAt2 string
	err = db.QueryRow("SELECT updated_at FROM skills WHERE name = ?", "ts").Scan(&updatedAt2)
	if err != nil {
		t.Fatalf("query updated_at: %v", err)
	}

	if updatedAt2 == updatedAt1 {
		t.Errorf("expected updated_at to change after older version publish, got %s both times", updatedAt1)
	}
}

func TestRepository_GetSkillContent(t *testing.T) {
	setup := func(t *testing.T) (*sql.DB, *sqlitestore.Repository) {
		t.Helper()
		db := openTestDB(t)
		repo := sqlitestore.New(db)
		err := repo.CreateSkillVersion(context.Background(),
			&skillctlv1.Skill{Name: "my-skill", Description: "test", Owner: "user-123", Tags: []string{"go"}},
			&skillctlv1.SkillVersion{Version: "1.0.0", PublishedBy: "u@ex.com", Digest: "sha256:abc", SizeBytes: 13, Changelog: "Initial"},
			[]byte("skill content"),
		)
		if err != nil {
			t.Fatalf("setup: %v", err)
		}
		return db, repo
	}

	tests := []struct {
		name        string
		skillName   string
		version     string
		digest      string
		wantContent string
		wantErr     error
	}{
		{
			name:        "get by version",
			skillName:   "my-skill",
			version:     "1.0.0",
			wantContent: "skill content",
		},
		{
			name:        "get latest when version empty",
			skillName:   "my-skill",
			version:     "",
			wantContent: "skill content",
		},
		{
			name:      "nonexistent skill",
			skillName: "nope",
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
			digest:      "sha256:abc",
			wantContent: "skill content",
		},
		{
			name:      "mismatched digest rejected",
			skillName: "my-skill",
			version:   "1.0.0",
			digest:    "sha256:wrong",
			wantErr:   store.ErrDigestMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, repo := setup(t)
			content, ver, err := repo.GetSkillContent(context.Background(), tt.skillName, tt.version, tt.digest)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(content) != tt.wantContent {
				t.Errorf("expected content %q, got %q", tt.wantContent, content)
			}
			if ver == nil {
				t.Fatal("expected version metadata, got nil")
			}
			if ver.Version != "1.0.0" {
				t.Errorf("expected version 1.0.0, got %s", ver.Version)
			}
		})
	}
}

func TestRepository_GetSkillContent_NullContent(t *testing.T) {
	db := openTestDB(t)
	// Insert a skill with no content (federated skill scenario)
	seedSkills(t, db)
	_, err := db.Exec(
		`INSERT INTO skill_versions (skill_name, version, digest, published_by) VALUES (?, ?, ?, ?)`,
		"data-pipeline", "1.3.0", "sha256:abc", "user@example.com",
	)
	if err != nil {
		t.Fatalf("insert version: %v", err)
	}

	repo := sqlitestore.New(db)
	_, _, err = repo.GetSkillContent(context.Background(), "data-pipeline", "1.3.0", "")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound for null content, got %v", err)
	}
}
