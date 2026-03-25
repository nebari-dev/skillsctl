// Package seed inserts default skills on server startup.
// If a skill+version already exists, it is silently skipped.
package seed

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/nebari-dev/skillsctl/backend/internal/store"
	skillsctlv1 "github.com/nebari-dev/skillsctl/gen/go/skillsctl/v1"
)

// Skill defines a skill to seed on startup.
type Skill struct {
	Name        string
	Description string
	Tags        []string
	Content     []byte
}

// Run seeds all provided skills into the repository at the given version.
// Existing skill+version pairs are silently skipped (idempotent).
func Run(ctx context.Context, repo store.Repository, version string, skills []Skill) error {
	if version == "" {
		version = "0.0.0"
	}

	for _, s := range skills {
		skill := &skillsctlv1.Skill{
			Name:        s.Name,
			Description: s.Description,
			Owner:       "system:seed",
			Tags:        s.Tags,
		}

		ver := &skillsctlv1.SkillVersion{
			Version:     version,
			Changelog:   "Seeded by server",
			PublishedBy: "system:seed",
		}

		err := repo.CreateSkillVersion(ctx, skill, ver, s.Content)
		if err != nil {
			if errors.Is(err, store.ErrAlreadyExists) {
				continue
			}
			if errors.Is(err, store.ErrPermissionDenied) {
				log.Printf("seed: skipping %s (owned by another user)", s.Name)
				continue
			}
			return fmt.Errorf("seed skill %s: %w", s.Name, err)
		}
		log.Printf("seed: seeded %s@%s", s.Name, version)
	}

	return nil
}
