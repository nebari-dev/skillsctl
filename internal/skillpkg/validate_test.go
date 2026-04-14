package skillpkg

import "testing"

func TestIsTarball(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want bool
	}{
		{"gzip magic", []byte{0x1f, 0x8b, 0x08, 0x00}, true},
		{"plain markdown", []byte("# SKILL\n"), false},
		{"empty", nil, false},
		{"one byte", []byte{0x1f}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTarball(tt.in); got != tt.want {
				t.Errorf("IsTarball(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
