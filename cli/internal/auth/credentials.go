package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// CachedToken holds a cached OIDC token and its expiry.
type CachedToken struct {
	IDToken string    `json:"id_token"`
	Expiry  time.Time `json:"expiry"`
}

// DefaultCredentialsPath returns ~/.config/skillsctl/credentials.json.
func DefaultCredentialsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "skillsctl", "credentials.json")
}

// FarFuture returns a time far in the future, useful for test tokens.
func FarFuture() time.Time {
	return time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
}

// SaveToken writes the token to the given path with 0600 permissions.
// Creates parent directories with 0700 permissions if needed.
func SaveToken(path string, tok *CachedToken) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(tok, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// LoadToken reads a cached token from the given path.
// Returns (nil, nil) if the file is missing, malformed, or the token is expired.
// Use LoadTokenRaw if you need to distinguish expired from missing.
func LoadToken(path string) (*CachedToken, error) {
	tok, err := LoadTokenRaw(path)
	if err != nil || tok == nil {
		return nil, nil
	}
	if time.Now().After(tok.Expiry) {
		return nil, nil
	}
	return tok, nil
}

// LoadTokenRaw reads a cached token regardless of expiry.
// Returns (nil, nil) if the file is missing or malformed.
// Returns the token even if expired (caller checks expiry).
func LoadTokenRaw(path string) (*CachedToken, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, nil
	}
	var tok CachedToken
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, nil
	}
	if tok.IDToken == "" {
		return nil, nil
	}
	return &tok, nil
}

// DeleteToken removes the credentials file. No error if missing.
func DeleteToken(path string) {
	os.Remove(path)
}
