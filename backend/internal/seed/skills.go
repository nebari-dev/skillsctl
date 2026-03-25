package seed

// DefaultSkills returns the built-in skills that should be seeded on startup.
// Content is loaded at build time and passed in from main.go.
func DefaultSkills(skillsctlUsageContent []byte) []Skill {
	return []Skill{
		{
			Name:        "skillsctl-usage",
			Description: "Guide for discovering, installing, and publishing Claude Code skills via the skillsctl CLI",
			Tags:        []string{"cli", "getting-started"},
			Content:     skillsctlUsageContent,
		},
	}
}
