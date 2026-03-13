package store

import (
	"context"
	"fmt"

	skillctlv1 "github.com/nebari-dev/skillctl/gen/go/skillctl/v1"
)

// Memory is an in-memory Repository for local development and testing.
type Memory struct {
	skills []*skillctlv1.Skill
}

var _ Repository = (*Memory)(nil)

// NewMemory creates an in-memory store pre-populated with the given skills.
func NewMemory(skills []*skillctlv1.Skill) *Memory {
	if skills == nil {
		skills = []*skillctlv1.Skill{}
	}
	return &Memory{skills: skills}
}

func (m *Memory) ListSkills(_ context.Context, tags []string, sourceFilter skillctlv1.SkillSource, _ int32, pageToken string) ([]*skillctlv1.Skill, string, error) {
	if pageToken != "" {
		return nil, "", ErrPaginationNotSupported
	}

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
	for _, s := range m.skills {
		if s.Name == name {
			return s, nil, nil
		}
	}
	return nil, nil, fmt.Errorf("%w: %s", ErrNotFound, name)
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
