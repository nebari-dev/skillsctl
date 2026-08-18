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

// Outcome describes what happened to one skill during a seed run.
type Outcome string

const (
	// OutcomeSeeded means this version was published by this run.
	OutcomeSeeded Outcome = "seeded"
	// OutcomeAlreadyPresent means the version existed, so nothing was written.
	// The content already in the registry is whichever binary published it
	// first, which is not necessarily this one.
	OutcomeAlreadyPresent Outcome = "already-present"
	// OutcomeNotOwner means the skill is owned by another publisher, so the
	// embedded copy can no longer update it.
	OutcomeNotOwner Outcome = "not-owner"
)

// Result records the fate of one skill in a seed run.
type Result struct {
	Name    string  `json:"name"`
	Version string  `json:"version"`
	Outcome Outcome `json:"outcome"`
}

// Run seeds all provided skills into the repository at the given version and
// reports what happened to each. Existing skill+version pairs are left alone,
// so a run is idempotent. Results are returned even when an error ends the run
// early, covering the skills processed up to that point.
func Run(ctx context.Context, repo store.Repository, version string, skills []Skill) ([]Result, error) {
	if version == "" {
		version = "0.0.0"
	}

	results := make([]Result, 0, len(skills))

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
				log.Printf("seed: %s@%s already present, skipping", s.Name, version)
				results = append(results, Result{Name: s.Name, Version: version, Outcome: OutcomeAlreadyPresent})
				continue
			}
			if errors.Is(err, store.ErrPermissionDenied) {
				log.Printf("seed: skipping %s (owned by another user)", s.Name)
				results = append(results, Result{Name: s.Name, Version: version, Outcome: OutcomeNotOwner})
				continue
			}
			return results, fmt.Errorf("seed skill %s: %w", s.Name, err)
		}
		log.Printf("seed: seeded %s@%s", s.Name, version)
		results = append(results, Result{Name: s.Name, Version: version, Outcome: OutcomeSeeded})
	}

	return results, nil
}
