package store

import (
	"context"
	"errors"

	skillsctlv1 "github.com/nebari-dev/skillsctl/gen/go/skillsctl/v1"
)

// ErrNotFound is returned when a requested skill does not exist.
var ErrNotFound = errors.New("skill not found")

// ErrPaginationNotSupported is returned when a pageToken is provided but
// the implementation does not yet support cursor-based pagination.
var ErrPaginationNotSupported = errors.New("pagination not yet supported")

var ErrAlreadyExists = errors.New("already exists")
var ErrPermissionDenied = errors.New("permission denied")
var ErrDigestMismatch = errors.New("digest mismatch")

// Repository defines the interface for skill persistence.
// This is the data abstraction layer between database implementations and
// application logic. All database access flows through this interface.
//
// Implementations: Memory (dev/test), SQLite (production).
type Repository interface {
	ListSkills(ctx context.Context, tags []string, sourceFilter skillsctlv1.SkillSource, pageSize int32, pageToken string) ([]*skillsctlv1.Skill, string, error)
	GetSkill(ctx context.Context, name string) (*skillsctlv1.Skill, []*skillsctlv1.SkillVersion, error)
	CreateSkillVersion(ctx context.Context, skill *skillsctlv1.Skill, version *skillsctlv1.SkillVersion, content []byte) error
	GetSkillContent(ctx context.Context, name string, version string, digest string) ([]byte, *skillsctlv1.SkillVersion, error)
}
