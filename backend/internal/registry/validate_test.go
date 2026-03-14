package registry

import (
	"strings"
	"testing"
)

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid short", input: "go", wantErr: false},
		{name: "valid with hyphens", input: "my-skill", wantErr: false},
		{name: "valid 64 chars", input: strings.Repeat("a", 64), wantErr: false},
		{name: "too short", input: "a", wantErr: true},
		{name: "too long", input: strings.Repeat("a", 65), wantErr: true},
		{name: "uppercase", input: "MySkill", wantErr: true},
		{name: "starts with hyphen", input: "-skill", wantErr: true},
		{name: "ends with hyphen", input: "skill-", wantErr: true},
		{name: "contains underscore", input: "my_skill", wantErr: true},
		{name: "contains space", input: "my skill", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid", input: "1.2.3", wantErr: false},
		{name: "valid with v prefix", input: "v1.2.3", wantErr: false},
		{name: "valid prerelease", input: "1.0.0-beta.1", wantErr: false},
		{name: "valid with build", input: "1.0.0+build.123", wantErr: false},
		{name: "empty", input: "", wantErr: true},
		{name: "not semver", input: "1.2", wantErr: true},
		{name: "garbage", input: "not-a-version", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVersion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateVersion(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateTags(t *testing.T) {
	tests := []struct {
		name    string
		tags    []string
		wantErr bool
	}{
		{name: "valid", tags: []string{"go", "testing"}, wantErr: false},
		{name: "empty list", tags: nil, wantErr: false},
		{name: "too many", tags: make([]string, 21), wantErr: true},
		{name: "tag too long", tags: []string{strings.Repeat("a", 65)}, wantErr: true},
		{name: "uppercase tag", tags: []string{"Go"}, wantErr: true},
		{name: "tag with spaces", tags: []string{"my tag"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "too many" {
				for i := range tt.tags {
					tt.tags[i] = "tag"
				}
			}
			err := validateTags(tt.tags)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTags(%v) error = %v, wantErr %v", tt.tags, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePublishRequest(t *testing.T) {
	validContent := []byte("# My Skill\nDoes stuff")
	tests := []struct {
		name        string
		skillName   string
		version     string
		description string
		tags        []string
		content     []byte
		wantErr     bool
	}{
		{
			name: "valid", skillName: "my-skill", version: "1.0.0",
			description: "A useful skill", tags: []string{"go"}, content: validContent,
		},
		{
			name: "empty content", skillName: "my-skill", version: "1.0.0",
			description: "A useful skill", content: nil, wantErr: true,
		},
		{
			name: "content too large", skillName: "my-skill", version: "1.0.0",
			description: "A useful skill", content: make([]byte, 1024*1024+1), wantErr: true,
		},
		{
			name: "empty description", skillName: "my-skill", version: "1.0.0",
			description: "", content: validContent, wantErr: true,
		},
		{
			name: "description too long", skillName: "my-skill", version: "1.0.0",
			description: strings.Repeat("a", 2001), content: validContent, wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePublishRequest(tt.skillName, tt.version, tt.description, tt.tags, tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePublishRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
