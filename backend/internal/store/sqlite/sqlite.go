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

	skillctlv1 "github.com/nebari-dev/skillctl/gen/go/skillctl/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/nebari-dev/skillctl/backend/internal/store"
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
		db.Close()
		return nil, fmt.Errorf("ping sqlite %s: %w", path, err)
	}

	return db, nil
}

func (r *Repository) ListSkills(ctx context.Context, tags []string, sourceFilter skillctlv1.SkillSource, pageSize int32, pageToken string) ([]*skillctlv1.Skill, string, error) {
	if pageToken != "" {
		return nil, "", store.ErrPaginationNotSupported
	}

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

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list skills: %w", err)
	}
	defer rows.Close()

	var skills []*skillctlv1.Skill
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

func (r *Repository) GetSkill(ctx context.Context, name string) (*skillctlv1.Skill, []*skillctlv1.SkillVersion, error) {
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

	if publishedAt != "" {
		t, err := parseTimestamp(publishedAt)
		if err != nil {
			return nil, fmt.Errorf("parse published_at: %w", err)
		}
		v.PublishedAt = t
	}

	return v, nil
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
