package cmd

import "testing"

func TestValidateSkillName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid simple", input: "my-skill", wantErr: false},
		{name: "valid two chars", input: "ab", wantErr: false},
		{name: "valid with numbers", input: "test-skill-123", wantErr: false},
		{name: "valid all digits", input: "12", wantErr: false},
		{name: "path traversal", input: "../evil", wantErr: true},
		{name: "single char", input: "a", wantErr: true},
		{name: "uppercase", input: "My-Skill", wantErr: true},
		{name: "dots", input: "my.skill", wantErr: true},
		{name: "starts with hyphen", input: "-my-skill", wantErr: true},
		{name: "ends with hyphen", input: "my-skill-", wantErr: true},
		{name: "empty string", input: "", wantErr: true},
		{name: "spaces", input: "my skill", wantErr: true},
		{name: "slashes", input: "my/skill", wantErr: true},
		{name: "too long", input: "aaaaaaaaaabbbbbbbbbbccccccccccddddddddddeeeeeeeeeeffffffffff01234", wantErr: true},
		{name: "max length valid", input: "aaaaaaaaaabbbbbbbbbbccccccccccddddddddddeeeeeeeeeefffffffff01234", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSkillName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSkillName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}
