package store

import (
	"context"
	"errors"

	skillctlv1 "github.com/nebari-dev/skillctl/gen/go/skillctl/v1"
)

// ErrNotFound is returned when a requested skill does not exist.
var ErrNotFound = errors.New("skill not found")

// ErrPaginationNotSupported is returned when a pageToken is provided but
// the implementation does not yet support cursor-based pagination.
var ErrPaginationNotSupported = errors.New("pagination not yet supported")

// Repository defines the interface for skill persistence.
// This is the data abstraction layer between database implementations and
// application logic. All database access flows through this interface.
//
// Implementations: Memory (dev/test), SQLite (production).
type Repository interface {
	ListSkills(ctx context.Context, tags []string, sourceFilter skillctlv1.SkillSource, pageSize int32, pageToken string) ([]*skillctlv1.Skill, string, error)
	GetSkill(ctx context.Context, name string) (*skillctlv1.Skill, []*skillctlv1.SkillVersion, error)
}
