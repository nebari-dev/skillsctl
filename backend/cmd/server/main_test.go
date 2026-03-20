package main

import "testing"

func TestIsDevMode(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want bool
	}{
		{name: "true lowercase", val: "true", want: true},
		{name: "TRUE uppercase", val: "TRUE", want: true},
		{name: "True mixed", val: "True", want: true},
		{name: "1", val: "1", want: true},
		{name: "yes", val: "yes", want: true},
		{name: "YES", val: "YES", want: true},
		{name: "empty", val: "", want: false},
		{name: "false", val: "false", want: false},
		{name: "0", val: "0", want: false},
		{name: "no", val: "no", want: false},
		{name: "random", val: "something", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DEV_MODE", tt.val)
			if got := isDevMode(); got != tt.want {
				t.Errorf("isDevMode() with DEV_MODE=%q = %v, want %v", tt.val, got, tt.want)
			}
		})
	}
}
