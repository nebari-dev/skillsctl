package store

import (
	"context"
	"fmt"
	"sync"

	skillctlv1 "github.com/nebari-dev/skillctl/gen/go/skillctl/v1"
	"golang.org/x/mod/semver"
)

type memoryVersion struct {
	meta    *skillctlv1.SkillVersion
	content []byte
}

// Memory is an in-memory Repository for local development and testing.
type Memory struct {
	mu       sync.Mutex
	skills   []*skillctlv1.Skill
	versions map[string][]memoryVersion // keyed by skill name
}

var _ Repository = (*Memory)(nil)

// NewMemory creates an in-memory store pre-populated with the given skills.
func NewMemory(skills []*skillctlv1.Skill) *Memory {
	if skills == nil {
		skills = []*skillctlv1.Skill{}
	}
	return &Memory{
		skills:   skills,
		versions: make(map[string][]memoryVersion),
	}
}

func (m *Memory) ListSkills(_ context.Context, tags []string, sourceFilter skillctlv1.SkillSource, _ int32, pageToken string) ([]*skillctlv1.Skill, string, error) {
	if pageToken != "" {
		return nil, "", ErrPaginationNotSupported
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var result []*skillctlv1.Skill
	for _, s := range m.skills {
		if sourceFilter != skillctlv1.SkillSource_SKILL_SOURCE_UNSPECIFIED && s.Source != sourceFilter {
			continue
		}
		if len(tags) > 0 && !hasAnyTag(s.Tags, tags) {
			continue
		}
		result = append(result, s)
	}
	return result, "", nil
}

func (m *Memory) GetSkill(_ context.Context, name string) (*skillctlv1.Skill, []*skillctlv1.SkillVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, s := range m.skills {
		if s.Name == name {
			var vers []*skillctlv1.SkillVersion
			for _, v := range m.versions[name] {
				vers = append(vers, v.meta)
			}
			return s, vers, nil
		}
	}
	return nil, nil, fmt.Errorf("%w: %s", ErrNotFound, name)
}

func (m *Memory) CreateSkillVersion(_ context.Context, skill *skillctlv1.Skill, version *skillctlv1.SkillVersion, content []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Look for an existing skill with this name.
	var existing *skillctlv1.Skill
	for _, s := range m.skills {
		if s.Name == skill.Name {
			existing = s
			break
		}
	}

	if existing != nil {
		// Owner check.
		if existing.Owner != skill.Owner {
			return fmt.Errorf("%w: not the owner of skill %q", ErrPermissionDenied, skill.Name)
		}

		// Duplicate version check.
		for _, v := range m.versions[skill.Name] {
			if v.meta.Version == version.Version {
				return fmt.Errorf("%w: version %s of skill %q", ErrAlreadyExists, version.Version, skill.Name)
			}
		}

		// Update latest_version only if the new version is semver-greater.
		if compareSemver(version.Version, existing.LatestVersion) > 0 {
			existing.LatestVersion = version.Version
		}
	} else {
		// First publish: create a new skill entry.
		newSkill := &skillctlv1.Skill{
			Name:          skill.Name,
			Description:   skill.Description,
			Owner:         skill.Owner,
			Tags:          skill.Tags,
			LatestVersion: version.Version,
			Source:        skillctlv1.SkillSource_SKILL_SOURCE_INTERNAL,
		}
		m.skills = append(m.skills, newSkill)
	}

	m.versions[skill.Name] = append(m.versions[skill.Name], memoryVersion{
		meta:    version,
		content: content,
	})

	return nil
}

func (m *Memory) GetSkillContent(_ context.Context, name string, version string, digest string) ([]byte, *skillctlv1.SkillVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find the skill.
	var skill *skillctlv1.Skill
	for _, s := range m.skills {
		if s.Name == name {
			skill = s
			break
		}
	}
	if skill == nil {
		return nil, nil, fmt.Errorf("%w: %s", ErrNotFound, name)
	}

	// If version is empty, resolve to latest.
	if version == "" {
		version = skill.LatestVersion
	}

	// Find the version.
	for _, v := range m.versions[name] {
		if v.meta.Version == version {
			if v.content == nil {
				return nil, nil, fmt.Errorf("%w: content for %s@%s", ErrNotFound, name, version)
			}
			if digest != "" && v.meta.Digest != digest {
				return nil, nil, fmt.Errorf("%w: expected %s, got %s", ErrDigestMismatch, digest, v.meta.Digest)
			}
			return v.content, v.meta, nil
		}
	}

	return nil, nil, fmt.Errorf("%w: version %s of skill %s", ErrNotFound, version, name)
}

func compareSemver(a, b string) int {
	if a != "" && a[0] != 'v' {
		a = "v" + a
	}
	if b != "" && b[0] != 'v' {
		b = "v" + b
	}
	return semver.Compare(a, b)
}

func hasAnyTag(skillTags, filterTags []string) bool {
	tagSet := make(map[string]struct{}, len(skillTags))
	for _, t := range skillTags {
		tagSet[t] = struct{}{}
	}
	for _, t := range filterTags {
		if _, ok := tagSet[t]; ok {
			return true
		}
	}
	return false
}
