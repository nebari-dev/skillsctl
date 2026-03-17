package cmd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nebari-dev/skillctl/cli/cmd"
	cliauth "github.com/nebari-dev/skillctl/cli/internal/auth"
)

// testJWT has payload {"sub":"user-1","email":"test@example.com","exp":9999999999}
const authTestJWT = "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyLTEiLCJlbWFpbCI6InRlc3RAZXhhbXBsZS5jb20iLCJleHAiOjk5OTk5OTk5OTl9.sig"

func mockFullOIDCServer(t *testing.T) *httptest.Server {
	t.Helper()
	var tokenCalls atomic.Int32

	mux := http.NewServeMux()
	ts := httptest.NewUnstartedServer(mux)
	ts.Start()

	mux.HandleFunc("/auth/config", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"enabled":    true,
			"issuer_url": ts.URL,
			"client_id":  "test-client",
		})
	})
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"device_authorization_endpoint": ts.URL + "/device",
			"token_endpoint":               ts.URL + "/token",
		})
	})
	mux.HandleFunc("/device", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"device_code":      "test-code",
			"user_code":        "TEST-CODE",
			"verification_uri": ts.URL + "/verify",
			"expires_in":       300,
			"interval":         1,
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		tokenCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"id_token": authTestJWT,
		})
	})

	t.Cleanup(ts.Close)
	return ts
}

func TestAuthLogin(t *testing.T) {
	ts := mockFullOIDCServer(t)

	tmpDir := t.TempDir()
	credsPath := filepath.Join(tmpDir, "credentials.json")

	var outBuf, errBuf bytes.Buffer
	root := cmd.NewRootCmd()
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"auth", "login",
		"--api-url", ts.URL,
		"--credentials-path", credsPath,
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := outBuf.String()
	if !strings.Contains(output, "test@example.com") {
		t.Errorf("expected email in output, got:\n%s", output)
	}

	// Verify credentials were saved
	tok, err := cliauth.LoadToken(credsPath)
	if err != nil {
		t.Fatalf("load token: %v", err)
	}
	if tok == nil {
		t.Fatal("expected saved token")
	}
}

func TestAuthLogin_AuthDisabled(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/config", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"enabled": false})
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	tmpDir := t.TempDir()
	credsPath := filepath.Join(tmpDir, "credentials.json")

	var buf bytes.Buffer
	root := cmd.NewRootCmd()
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{
		"auth", "login",
		"--api-url", ts.URL,
		"--credentials-path", credsPath,
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "does not require authentication") {
		t.Errorf("expected disabled message, got:\n%s", buf.String())
	}
}

func TestAuthStatus_LoggedIn(t *testing.T) {
	tmpDir := t.TempDir()
	credsPath := filepath.Join(tmpDir, "credentials.json")

	cliauth.SaveToken(credsPath, &cliauth.CachedToken{
		IDToken: authTestJWT,
		Expiry:  cliauth.FarFuture(),
	})

	var buf bytes.Buffer
	root := cmd.NewRootCmd()
	root.SetOut(&buf)
	root.SetArgs([]string{"auth", "status", "--credentials-path", credsPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "test@example.com") {
		t.Errorf("expected email, got:\n%s", buf.String())
	}
}

func TestAuthStatus_NotLoggedIn(t *testing.T) {
	tmpDir := t.TempDir()
	credsPath := filepath.Join(tmpDir, "nonexistent.json")

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"auth", "status", "--credentials-path", credsPath})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for not logged in")
	}
	if !strings.Contains(err.Error(), "not logged in") {
		t.Errorf("expected 'not logged in', got: %v", err)
	}
}

func TestAuthStatus_Expired(t *testing.T) {
	tmpDir := t.TempDir()
	credsPath := filepath.Join(tmpDir, "credentials.json")

	cliauth.SaveToken(credsPath, &cliauth.CachedToken{
		IDToken: authTestJWT,
		Expiry:  time.Now().Add(-time.Hour),
	})

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"auth", "status", "--credentials-path", credsPath})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for expired token")
	}
	if !strings.Contains(err.Error(), "session expired") {
		t.Errorf("expected 'session expired', got: %v", err)
	}
}

func TestAuthLogout(t *testing.T) {
	tmpDir := t.TempDir()
	credsPath := filepath.Join(tmpDir, "credentials.json")
	os.WriteFile(credsPath, []byte(`{"id_token":"x"}`), 0600)

	var buf bytes.Buffer
	root := cmd.NewRootCmd()
	root.SetOut(&buf)
	root.SetArgs([]string{"auth", "logout", "--credentials-path", credsPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "Logged out") {
		t.Errorf("expected 'Logged out', got:\n%s", buf.String())
	}

	if _, err := os.Stat(credsPath); !os.IsNotExist(err) {
		t.Error("expected credentials file to be deleted")
	}
}
