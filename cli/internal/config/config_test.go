package config_test

import (
	"strings"
	"testing"

	"github.com/openteams-ai/skill-share/cli/internal/config"
)

func TestDefaults(t *testing.T) {
	cfg := config.Load()

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"api_url default", cfg.APIURL, "http://localhost:8080"},
		{"skills_dir ends with .claude/skills", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "skills_dir ends with .claude/skills" {
				if !strings.HasSuffix(cfg.SkillsDir, ".claude/skills") {
					t.Errorf("skills_dir %q does not end with .claude/skills", cfg.SkillsDir)
				}
				return
			}
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}
