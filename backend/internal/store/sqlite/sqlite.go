// Package sqlite provides a SQLite-backed implementation of store.Repository.
//
// Callers must register the SQLite driver before using this package by
// importing the driver for side effects:
//
//	import _ "modernc.org/sqlite"
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/mod/semver"
	"google.golang.org/protobuf/types/known/timestamppb"
	sqlite "modernc.org/sqlite"

	skillsctlv1 "github.com/nebari-dev/skillsctl/gen/go/skillsctl/v1"

	"github.com/nebari-dev/skillsctl/backend/internal/store"
)

// Repository is a SQLite-backed store.Repository implementation.
type Repository struct {
	db *sql.DB
}

var _ store.Repository = (*Repository)(nil)

// New creates a SQLite-backed repository using the given database connection.
// The caller is responsible for running migrations before calling this.
func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Open opens a SQLite database at the given path with WAL mode and recommended
// pragmas. Sets MaxOpenConns=1 since SQLite is single-writer. Pings the
// database to verify the connection is usable.
//
// Callers must register the SQLite driver before calling Open:
//
//	import _ "modernc.org/sqlite"
func Open(path string) (*sql.DB, error) {
	dsn := path + "?" + strings.Join([]string{
		"_pragma=journal_mode=WAL",
		"_pragma=busy_timeout=5000",
		"_pragma=foreign_keys=ON",
		"_pragma=synchronous=NORMAL",
	}, "&")
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite %s: %w", path, err)
	}

	return db, nil
}

//nolint:gosec // G202: query is built with string concatenation but all user values use ? parameters
func (r *Repository) ListSkills(ctx context.Context, tags []string, sourceFilter skillsctlv1.SkillSource, pageSize int32, pageToken string) ([]*skillsctlv1.Skill, string, error) {
	if pageToken != "" {
		return nil, "", store.ErrPaginationNotSupported
	}

	query := `SELECT name, description, owner, tags, latest_version, install_count, created_at, updated_at, source, marketplace_id, upstream_url FROM skills`
	var conditions []string
	var args []any

	if sourceFilter != skillsctlv1.SkillSource_SKILL_SOURCE_UNSPECIFIED {
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

	const defaultPageSize = 100
	const maxPageSize = 500
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	query += " LIMIT ?"
	args = append(args, pageSize)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list skills: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var skills []*skillsctlv1.Skill
	for rows.Next() {
		skill, err := scanSkillFields(rows)
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

func (r *Repository) GetSkill(ctx context.Context, name string) (*skillsctlv1.Skill, []*skillsctlv1.SkillVersion, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT name, description, owner, tags, latest_version, install_count, created_at, updated_at, source, marketplace_id, upstream_url FROM skills WHERE name = ?`,
		name,
	)
	skill, err := scanSkillFields(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, fmt.Errorf("%w: %s", store.ErrNotFound, name)
		}
		return nil, nil, fmt.Errorf("get skill %s: %w", name, err)
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT version, changelog, oci_ref, digest, size_bytes, published_by, published_at, draft FROM skill_versions WHERE skill_name = ? ORDER BY published_at DESC`,
		name,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("get skill versions %s: %w", name, err)
	}
	defer func() { _ = rows.Close() }()

	var versions []*skillsctlv1.SkillVersion
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

func (r *Repository) CreateSkillVersion(ctx context.Context, skill *skillsctlv1.Skill, version *skillsctlv1.SkillVersion, content []byte) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Check if skill exists.
	var existingOwner string
	var existingLatest string
	err = tx.QueryRowContext(ctx, "SELECT owner, latest_version FROM skills WHERE name = ?", skill.Name).Scan(&existingOwner, &existingLatest)

	if errors.Is(err, sql.ErrNoRows) {
		// First publish - create skill.
		tagsJSON, jsonErr := json.Marshal(skill.Tags)
		if jsonErr != nil {
			return fmt.Errorf("marshal tags: %w", jsonErr)
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO skills (name, description, owner, tags, latest_version, source) VALUES (?, ?, ?, ?, ?, ?)`,
			skill.Name, skill.Description, skill.Owner, string(tagsJSON), version.Version,
			int(skillsctlv1.SkillSource_SKILL_SOURCE_INTERNAL),
		)
		if err != nil {
			return fmt.Errorf("insert skill: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("check skill: %w", err)
	} else {
		// Existing skill - check ownership.
		if existingOwner != skill.Owner {
			return fmt.Errorf("%w: not the owner of skill %q", store.ErrPermissionDenied, skill.Name)
		}
		// Update latest_version only if semver-greater.
		if compareSemver(version.Version, existingLatest) > 0 {
			_, err = tx.ExecContext(ctx,
				`UPDATE skills SET latest_version = ? WHERE name = ?`,
				version.Version, skill.Name,
			)
			if err != nil {
				return fmt.Errorf("update latest_version: %w", err)
			}
		}
		// Always update updated_at on every successful publish.
		_, err = tx.ExecContext(ctx,
			`UPDATE skills SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE name = ?`,
			skill.Name,
		)
		if err != nil {
			return fmt.Errorf("update updated_at: %w", err)
		}
		// Update description and tags if provided.
		if skill.Description != "" {
			_, err = tx.ExecContext(ctx, `UPDATE skills SET description = ? WHERE name = ?`, skill.Description, skill.Name)
			if err != nil {
				return fmt.Errorf("update description: %w", err)
			}
		}
		if len(skill.Tags) > 0 {
			tagsJSON, jsonErr := json.Marshal(skill.Tags)
			if jsonErr != nil {
				return fmt.Errorf("marshal tags: %w", jsonErr)
			}
			_, err = tx.ExecContext(ctx, `UPDATE skills SET tags = ? WHERE name = ?`, string(tagsJSON), skill.Name)
			if err != nil {
				return fmt.Errorf("update tags: %w", err)
			}
		}
	}

	// Insert version - PRIMARY KEY constraint enforces immutability.
	_, err = tx.ExecContext(ctx,
		`INSERT INTO skill_versions (skill_name, version, published_by, changelog, digest, size_bytes, content) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		skill.Name, version.Version, version.PublishedBy, version.Changelog, version.Digest, version.SizeBytes, content,
	)
	if err != nil {
		if isConstraintViolation(err) {
			return fmt.Errorf("%w: version %s of %s", store.ErrAlreadyExists, version.Version, skill.Name)
		}
		return fmt.Errorf("insert version: %w", err)
	}

	return tx.Commit()
}

func (r *Repository) GetSkillContent(ctx context.Context, name string, version string, digest string) ([]byte, *skillsctlv1.SkillVersion, error) {
	// Build query - use a subquery to resolve latest version atomically when
	// version is empty, avoiding a TOCTOU race between two separate reads.
	query := `SELECT version, changelog, oci_ref, digest, size_bytes, published_by, published_at, draft, content
		 FROM skill_versions WHERE skill_name = ?`
	var args []any
	args = append(args, name)

	if version == "" {
		query += ` AND version = (SELECT latest_version FROM skills WHERE name = ?)`
		args = append(args, name)
	} else {
		query += ` AND version = ?`
		args = append(args, version)
	}

	var (
		v           skillsctlv1.SkillVersion
		content     []byte
		publishedAt string
		draft       int
	)
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&v.Version, &v.Changelog, &v.OciRef, &v.Digest, &v.SizeBytes, &v.PublishedBy, &publishedAt, &draft, &content,
	)

	if errors.Is(err, sql.ErrNoRows) {
		if version == "" {
			return nil, nil, fmt.Errorf("%w: %s (no version available)", store.ErrNotFound, name)
		}
		return nil, nil, fmt.Errorf("%w: %s@%s", store.ErrNotFound, name, version)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get skill content: %w", err)
	}

	v.Draft = draft != 0
	if publishedAt != "" {
		t, err := parseTimestamp(publishedAt)
		if err != nil {
			return nil, nil, fmt.Errorf("parse published_at: %w", err)
		}
		v.PublishedAt = t
	}

	if content == nil {
		return nil, nil, fmt.Errorf("%w: no content for %s@%s", store.ErrNotFound, name, version)
	}

	if digest != "" && v.Digest != digest {
		return nil, nil, fmt.Errorf("%w: expected %s, got %s", store.ErrDigestMismatch, digest, v.Digest)
	}

	return content, &v, nil
}

// isConstraintViolation checks if a SQLite error is a PRIMARY KEY or UNIQUE
// constraint violation using the typed error from modernc.org/sqlite.
// SQLITE_CONSTRAINT_PRIMARYKEY = 1555, SQLITE_CONSTRAINT_UNIQUE = 2067.
func isConstraintViolation(err error) bool {
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		code := sqliteErr.Code()
		return code == 1555 || code == 2067
	}
	return false
}

// compareSemver compares two semver strings, prepending "v" if absent.
// Returns negative if a < b, zero if a == b, positive if a > b.
func compareSemver(a, b string) int {
	if a != "" && a[0] != 'v' {
		a = "v" + a
	}
	if b != "" && b[0] != 'v' {
		b = "v" + b
	}
	return semver.Compare(a, b)
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanSkillFields(sc scanner) (*skillsctlv1.Skill, error) {
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

	skill := &skillsctlv1.Skill{
		Name:          name,
		Description:   desc,
		Owner:         owner,
		Tags:          tags,
		LatestVersion: version,
		InstallCount:  installs,
		Source:        validSkillSource(source),
		MarketplaceId: marketplaceID,
		UpstreamUrl:   upstreamURL,
	}

	if createdAt != "" {
		t, err := parseTimestamp(createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse created_at for %s: %w", name, err)
		}
		skill.CreatedAt = t
	}
	if updatedAt != "" {
		t, err := parseTimestamp(updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse updated_at for %s: %w", name, err)
		}
		skill.UpdatedAt = t
	}

	return skill, nil
}

func scanVersion(rows *sql.Rows) (*skillsctlv1.SkillVersion, error) {
	var (
		version, changelog, ociRef, digest, publishedBy string
		sizeBytes                                       int64
		publishedAt                                     string
		draft                                           int
	)
	if err := rows.Scan(&version, &changelog, &ociRef, &digest, &sizeBytes, &publishedBy, &publishedAt, &draft); err != nil {
		return nil, fmt.Errorf("scan version: %w", err)
	}

	v := &skillsctlv1.SkillVersion{
		Version:     version,
		Changelog:   changelog,
		OciRef:      ociRef,
		Digest:      digest,
		SizeBytes:   sizeBytes,
		PublishedBy: publishedBy,
		Draft:       draft != 0,
	}

	if publishedAt != "" {
		t, err := parseTimestamp(publishedAt)
		if err != nil {
			return nil, fmt.Errorf("parse published_at: %w", err)
		}
		v.PublishedAt = t
	}

	return v, nil
}

// validSkillSource returns the SkillSource for a known enum value, or
// SKILL_SOURCE_UNSPECIFIED if the integer doesn't map to a known value.
func validSkillSource(v int) skillsctlv1.SkillSource {
	if v < 0 || v > int(^int32(0)) {
		return skillsctlv1.SkillSource_SKILL_SOURCE_UNSPECIFIED
	}
	if _, ok := skillsctlv1.SkillSource_name[int32(v)]; ok { //nolint:gosec // overflow guarded above
		return skillsctlv1.SkillSource(v) //nolint:gosec // overflow guarded above
	}
	return skillsctlv1.SkillSource_SKILL_SOURCE_UNSPECIFIED
}

// parseTimestamp tries multiple time formats because the canonical format from
// SQLite's strftime('%Y-%m-%dT%H:%M:%fZ') includes milliseconds, but data may
// also be inserted manually or by goose seeds using simpler formats.
func parseTimestamp(s string) (*timestamppb.Timestamp, error) {
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
