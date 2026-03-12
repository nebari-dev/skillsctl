package store

import (
	"context"
	"errors"

	skillctlv1 "github.com/nebari-dev/skillctl/gen/go/skillctl/v1"
)

// ErrNotFound is returned when a requested skill does not exist.
var ErrNotFound = errors.New("skill not found")

// SkillStore defines the interface for skill persistence.
// Implementations: Memory (dev/test), SQLite (production).
type SkillStore interface {
	ListSkills(ctx context.Context, tags []string, sourceFilter skillctlv1.SkillSource, pageSize int32, pageToken string) ([]*skillctlv1.Skill, string, error)
	GetSkill(ctx context.Context, name string) (*skillctlv1.Skill, []*skillctlv1.SkillVersion, error)
}
