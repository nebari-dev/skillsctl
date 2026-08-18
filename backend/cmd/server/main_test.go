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

func TestResolveSeedVersion(t *testing.T) {
	tests := []struct {
		name       string
		build      string
		appVersion string
		want       string
		wantErr    bool
	}{
		{name: "stamped build, matching APP_VERSION", build: "0.2.2", appVersion: "0.2.2", want: "0.2.2"},
		{name: "stamped build, no APP_VERSION", build: "0.2.2", appVersion: "", want: "0.2.2"},
		{name: "stamped build, APP_VERSION ahead", build: "0.2.0", appVersion: "0.2.1", wantErr: true},
		{name: "stamped build, APP_VERSION behind", build: "0.2.2", appVersion: "0.2.1", wantErr: true},
		{name: "dev build falls back to APP_VERSION", build: "dev", appVersion: "9.9.9", want: "9.9.9"},
		{name: "dev build with no APP_VERSION", build: "dev", appVersion: "", want: "0.0.0"},
		{name: "empty build treated as unstamped", build: "", appVersion: "1.2.3", want: "1.2.3"},
		{name: "empty build and no APP_VERSION", build: "", appVersion: "", want: "0.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveSeedVersion(tt.build, tt.appVersion)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolveSeedVersion(%q, %q) = %q, want error", tt.build, tt.appVersion, got)
				}
				if got != "" {
					t.Errorf("resolveSeedVersion(%q, %q) returned version %q alongside error, want empty", tt.build, tt.appVersion, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveSeedVersion(%q, %q) unexpected error: %v", tt.build, tt.appVersion, err)
			}
			if got != tt.want {
				t.Errorf("resolveSeedVersion(%q, %q) = %q, want %q", tt.build, tt.appVersion, got, tt.want)
			}
		})
	}
}
