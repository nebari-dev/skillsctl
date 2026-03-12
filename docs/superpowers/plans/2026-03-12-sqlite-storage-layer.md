# SQLite Storage Layer Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the in-memory store with a SQLite-backed store using goose migrations, keeping the existing `SkillStore` interface and all tests passing.

**Architecture:** SQLite via `modernc.org/sqlite` (pure Go, no CGO) accessed through `database/sql`. Goose manages schema migrations embedded in the binary. The SQLite store implements the existing `SkillStore` interface so it's a drop-in replacement. WAL mode enables concurrent readers with single-writer blocking. Connection pool is limited to 1 open connection since SQLite is single-writer; pragmas (foreign_keys, WAL) are set via DSN parameters so they apply to every connection.

**Tech Stack:** `modernc.org/sqlite`, `database/sql`, `pressly/goose/v3`, Go embed

**Spec:** `docs/superpowers/specs/2026-03-12-sqlite-storage-design.md`

**Scope notes:**
- This plan covers only `001_initial.sql` (skills + skill_versions). Migrations `002_federation.sql` and `003_audit_log.sql` are deferred to later plans.
- Pagination (`pageToken`) is not implemented in this plan - the interface accepts it but the SQLite store returns empty next tokens. This will be addressed when the CLI needs it.
- Tags use a JSON array in a TEXT column (chosen over join table from the two options in the spec). Queried via `json_each()`.

---

## File Structure

| File | Responsibility |
|---|---|
| `backend/internal/store/store.go` | Modify: update interface comment from "PostgreSQL" to "SQLite" |
| `backend/internal/store/sqlite.go` | Create: SQLite `SkillStore` implementation + `OpenSQLite` helper |
| `backend/internal/store/sqlite_test.go` | Create: tests for SQLite store (table-driven, same cases as memory_test plus SQLite-specific) |
| `backend/internal/store/migrations/001_initial.sql` | Create: skills + skill_versions tables |
| `backend/internal/store/migrations/migrate.go` | Create: embedded FS + RunMigrations helper |
| `backend/cmd/server/main.go` | Modify: read `DB_PATH` env var, open SQLite, run migrations, use SQLite store |

---

## Chunk 1: Migrations and Database Setup

### Task 1: Add dependencies

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Add modernc.org/sqlite and goose**

Run:
```bash
go get modernc.org/sqlite
go get github.com/pressly/goose/v3
```

- [ ] **Step 2: Verify build still passes**

Run: `go build ./...`
Expected: clean build, no errors

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add modernc.org/sqlite and goose/v3"
```

---

### Task 2: Create initial migration

**Files:**
- Create: `backend/internal/store/migrations/001_initial.sql`

- [ ] **Step 1: Write the migration file**

```sql
-- +goose Up
CREATE TABLE skills (
    name            TEXT PRIMARY KEY,
    description     TEXT NOT NULL,
    owner           TEXT NOT NULL,
    tags            TEXT NOT NULL DEFAULT '[]',
    latest_version  TEXT NOT NULL,
    install_count   INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    source          INTEGER NOT NULL DEFAULT 1,
    marketplace_id  TEXT NOT NULL DEFAULT '',
    upstream_url    TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_skills_updated ON skills(updated_at DESC);

CREATE TABLE skill_versions (
    skill_name    TEXT NOT NULL REFERENCES skills(name) ON DELETE CASCADE,
    version       TEXT NOT NULL,
    oci_ref       TEXT NOT NULL DEFAULT '',
    digest        TEXT NOT NULL DEFAULT '',
    published_by  TEXT NOT NULL DEFAULT '',
    changelog     TEXT NOT NULL DEFAULT '',
    size_bytes    INTEGER NOT NULL DEFAULT 0,
    draft         INTEGER NOT NULL DEFAULT 0,
    published_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY (skill_name, version)
);

-- +goose Down
DROP TABLE IF EXISTS skill_versions;
DROP TABLE IF EXISTS skills;
```

Notes:
- `tags` is a JSON array in a TEXT column (e.g., `'["cli","go"]'`), queryable via `json_each()`
- `source` is an integer matching the proto enum (1=INTERNAL, 2=FEDERATED)
- Timestamps are ISO 8601 strings (SQLite has no native datetime)
- `draft` uses INTEGER (0/1) since SQLite has no boolean type

- [ ] **Step 2: Commit**

```bash
git add backend/internal/store/migrations/001_initial.sql
git commit -m "feat: add initial SQLite migration for skills and skill_versions"
```

---

### Task 3: Create migration runner

**Files:**
- Create: `backend/internal/store/migrations/migrate.go`

- [ ] **Step 1: Write the migration runner**

```go
package migrations

import (
	"database/sql"
	"embed"

	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var fs embed.FS

// Run executes all pending migrations against the given database.
func Run(db *sql.DB) error {
	goose.SetBaseFS(fs)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}
	return goose.Up(db, ".")
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./backend/internal/store/migrations/`
Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add backend/internal/store/migrations/migrate.go
git commit -m "feat: add embedded goose migration runner"
```

---

## Chunk 2: SQLite Store Implementation

### Task 4: Write SQLite store implementation and tests

**Files:**
- Create: `backend/internal/store/sqlite.go`
- Create: `backend/internal/store/sqlite_test.go`
- Modify: `backend/internal/store/store.go` (update comment only)

- [ ] **Step 1: Update store.go interface comment**

In `backend/internal/store/store.go`, change line 14:
```go
// Implementations: Memory (dev/test), PostgreSQL (production).
```
to:
```go
// Implementations: Memory (dev/test), SQLite (production).
```

- [ ] **Step 2: Write the SQLite store implementation**

Create `backend/internal/store/sqlite.go`:

```go
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	skillctlv1 "github.com/nebari-dev/skillctl/gen/go/skillctl/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// SQLite is a SQLite-backed SkillStore for production use.
type SQLite struct {
	db *sql.DB
}

var _ SkillStore = (*SQLite)(nil)

// NewSQLite creates a SQLite-backed store using the given database connection.
// The caller is responsible for running migrations before calling this.
func NewSQLite(db *sql.DB) *SQLite {
	return &SQLite{db: db}
}

// OpenSQLite opens a SQLite database at the given path with WAL mode and
// recommended pragmas. Sets MaxOpenConns=1 since SQLite is single-writer,
// and configures pragmas via DSN so they apply to every connection.
func OpenSQLite(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s?_pragma=journal_mode%%3DWAL&_pragma=busy_timeout%%3D5000&_pragma=foreign_keys%%3DON&_pragma=synchronous%%3DNORMAL", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

func (s *SQLite) ListSkills(ctx context.Context, tags []string, sourceFilter skillctlv1.SkillSource, pageSize int32, pageToken string) ([]*skillctlv1.Skill, string, error) {
	query := `SELECT name, description, owner, tags, latest_version, install_count, created_at, updated_at, source, marketplace_id, upstream_url FROM skills`
	var conditions []string
	var args []any

	if sourceFilter != skillctlv1.SkillSource_SKILL_SOURCE_UNSPECIFIED {
		conditions = append(conditions, "source = ?")
		args = append(args, int(sourceFilter))
	}

	if len(tags) > 0 {
		// Match skills that have ANY of the requested tags.
		// Uses json_each() to unnest the JSON array in the tags column.
		placeholders := make([]string, len(tags))
		for i, tag := range tags {
			placeholders[i] = "?"
			args = append(args, tag)
		}
		conditions = append(conditions, fmt.Sprintf(
			"EXISTS (SELECT 1 FROM json_each(tags) WHERE json_each.value IN (%s))",
			strings.Join(placeholders, ","),
		))
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY updated_at DESC"

	if pageSize > 0 {
		query += fmt.Sprintf(" LIMIT %d", pageSize)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list skills: %w", err)
	}
	defer rows.Close()

	var skills []*skillctlv1.Skill
	for rows.Next() {
		skill, err := scanSkill(rows)
		if err != nil {
			return nil, "", err
		}
		skills = append(skills, skill)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("list skills rows: %w", err)
	}
	return skills, "", nil
}

func (s *SQLite) GetSkill(ctx context.Context, name string) (*skillctlv1.Skill, []*skillctlv1.SkillVersion, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT name, description, owner, tags, latest_version, install_count, created_at, updated_at, source, marketplace_id, upstream_url FROM skills WHERE name = ?`,
		name,
	)
	skill, err := scanSkillRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, fmt.Errorf("%w: %s", ErrNotFound, name)
		}
		return nil, nil, fmt.Errorf("get skill %s: %w", name, err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT version, changelog, oci_ref, digest, size_bytes, published_by, published_at, draft FROM skill_versions WHERE skill_name = ? ORDER BY published_at DESC`,
		name,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("get skill versions %s: %w", name, err)
	}
	defer rows.Close()

	var versions []*skillctlv1.SkillVersion
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, nil, err
		}
		versions = append(versions, v)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("get skill versions rows %s: %w", name, err)
	}

	return skill, versions, nil
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanSkillFields(sc scanner) (*skillctlv1.Skill, error) {
	var (
		name, desc, owner, tagsJSON, version string
		installs                             int64
		createdAt, updatedAt                 string
		source                               int
		marketplaceID, upstreamURL           string
	)
	if err := sc.Scan(&name, &desc, &owner, &tagsJSON, &version, &installs, &createdAt, &updatedAt, &source, &marketplaceID, &upstreamURL); err != nil {
		return nil, err
	}

	var tags []string
	if err := json.Unmarshal([]byte(tagsJSON), &tags); err != nil {
		return nil, fmt.Errorf("unmarshal tags for %s: %w", name, err)
	}

	skill := &skillctlv1.Skill{
		Name:          name,
		Description:   desc,
		Owner:         owner,
		Tags:          tags,
		LatestVersion: version,
		InstallCount:  installs,
		Source:        skillctlv1.SkillSource(source),
		MarketplaceId: marketplaceID,
		UpstreamUrl:   upstreamURL,
	}

	if t, err := parseTimestamp(createdAt); err == nil {
		skill.CreatedAt = t
	}
	if t, err := parseTimestamp(updatedAt); err == nil {
		skill.UpdatedAt = t
	}

	return skill, nil
}

func scanSkill(rows *sql.Rows) (*skillctlv1.Skill, error) {
	return scanSkillFields(rows)
}

func scanSkillRow(row *sql.Row) (*skillctlv1.Skill, error) {
	return scanSkillFields(row)
}

func scanVersion(rows *sql.Rows) (*skillctlv1.SkillVersion, error) {
	var (
		version, changelog, ociRef, digest, publishedBy string
		sizeBytes                                       int64
		publishedAt                                     string
		draft                                           int
	)
	if err := rows.Scan(&version, &changelog, &ociRef, &digest, &sizeBytes, &publishedBy, &publishedAt, &draft); err != nil {
		return nil, fmt.Errorf("scan version: %w", err)
	}

	v := &skillctlv1.SkillVersion{
		Version:     version,
		Changelog:   changelog,
		OciRef:      ociRef,
		Digest:      digest,
		SizeBytes:   sizeBytes,
		PublishedBy: publishedBy,
		Draft:       draft != 0,
	}

	if t, err := parseTimestamp(publishedAt); err == nil {
		v.PublishedAt = t
	}

	return v, nil
}

func parseTimestamp(s string) (*timestamppb.Timestamp, error) {
	// SQLite stores timestamps as ISO 8601 strings.
	// Try parsing with the format used by strftime in the migration.
	formats := []string{
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return timestamppb.New(t), nil
		}
	}
	return nil, fmt.Errorf("cannot parse timestamp %q", s)
}
```

- [ ] **Step 3: Write tests for the SQLite store**

Create `backend/internal/store/sqlite_test.go`. Tests use `store.OpenSQLite(":memory:")` to ensure pragmas (foreign_keys, etc.) are consistently set in tests just like production. Tests verify tag JSON round-trip integrity.

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -race ./backend/internal/store/... -v`
Expected: all tests pass (both Memory and SQLite tests)

- [ ] **Step 5: Commit**

```bash
git add backend/internal/store/sqlite.go backend/internal/store/sqlite_test.go backend/internal/store/store.go
git commit -m "feat: add SQLite SkillStore implementation with tests"
```

---

## Chunk 3: Wire Into Server

### Task 5: Update server main to use SQLite

**Files:**
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Update main.go to open SQLite and run migrations**

Replace the entire `main.go` with:

```go
package main

import (
	"log"
	"net/http"
	"os"

	_ "modernc.org/sqlite"

	"github.com/nebari-dev/skillctl/backend/internal/server"
	"github.com/nebari-dev/skillctl/backend/internal/store"
	"github.com/nebari-dev/skillctl/backend/internal/store/migrations"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "skillctl.db"
	}

	db, err := store.OpenSQLite(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := migrations.Run(db); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	skillStore := store.NewSQLite(db)
	srv := server.New(skillStore)

	log.Printf("starting server on :%s (db: %s)", port, dbPath)
	if err := http.ListenAndServe(":"+port, srv.Handler()); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
```

Note: `DB_PATH` env var configures the database path. Default is `skillctl.db` in the working directory. Use `:memory:` for ephemeral testing. WAL mode is silently ignored for `:memory:` databases but all other pragmas still apply.

- [ ] **Step 2: Verify build**

Run: `go build ./backend/cmd/server/`
Expected: clean build

- [ ] **Step 3: Smoke test - start server and hit healthz**

Run:
```bash
DB_PATH=":memory:" go run ./backend/cmd/server/ &
sleep 1
curl -s http://localhost:8080/healthz
kill %1
```
Expected: `ok`

- [ ] **Step 4: Run all tests to verify nothing is broken**

Run: `go test -race ./... -v`
Expected: all tests pass

- [ ] **Step 5: Commit**

```bash
git add backend/cmd/server/main.go
git commit -m "feat: wire SQLite store into server startup with auto-migration"
```
