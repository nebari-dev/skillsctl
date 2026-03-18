package cmd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nebari-dev/skillsctl/cli/cmd"
	cliauth "github.com/nebari-dev/skillsctl/cli/internal/auth"
	"github.com/nebari-dev/skillsctl/cli/internal/testutil"
)

// testJWT has payload {"sub":"user-1","email":"test@example.com","exp":9999999999}
const authTestJWT = "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyLTEiLCJlbWFpbCI6InRlc3RAZXhhbXBsZS5jb20iLCJleHAiOjk5OTk5OTk5OTl9.sig"

func mockFullOIDCServer(t *testing.T) *httptest.Server {
	t.Helper()

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

func TestAuthLogin_ThenPublishUsesToken(t *testing.T) {
	// Set up a combined OIDC + skillsctl stub server
	var gotAuth string

	mux := http.NewServeMux()
	ts := httptest.NewUnstartedServer(mux)
	ts.Start()
	t.Cleanup(ts.Close)

	// OIDC endpoints
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
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"id_token": authTestJWT,
		})
	})

	// Use a real stub server for ConnectRPC publish endpoint so proto content types work.
	// We'll use a separate server and capture auth there.
	pubStub := testutil.NewStubServer(t, nil)

	tmpDir := t.TempDir()
	credsPath := filepath.Join(tmpDir, "credentials.json")
	skillFile := filepath.Join(tmpDir, "skill.md")
	os.WriteFile(skillFile, []byte("# Test"), 0644)

	// Step 1: Login (talks to OIDC mock)
	loginRoot := cmd.NewRootCmd()
	loginRoot.SetOut(&bytes.Buffer{})
	loginRoot.SetErr(&bytes.Buffer{})
	loginRoot.SetArgs([]string{
		"auth", "login",
		"--api-url", ts.URL,
		"--credentials-path", credsPath,
	})
	if err := loginRoot.Execute(); err != nil {
		t.Fatalf("login: %v", err)
	}

	// Verify token was saved
	tok, _ := cliauth.LoadToken(credsPath)
	if tok == nil {
		t.Fatal("expected saved token after login")
	}

	// Step 2: Publish with the saved token (talks to stub server)
	// Use a custom HTTP handler that wraps the stub to capture the auth header
	captureMux := http.NewServeMux()
	captureMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		// Proxy to the real stub server
		proxyReq, _ := http.NewRequest(r.Method, pubStub.URL+r.URL.Path, r.Body)
		for k, v := range r.Header {
			proxyReq.Header[k] = v
		}
		resp, err := http.DefaultClient.Do(proxyReq)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer resp.Body.Close()
		for k, v := range resp.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(resp.StatusCode)
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		w.Write(buf.Bytes())
	})
	captureServer := httptest.NewServer(captureMux)
	t.Cleanup(captureServer.Close)

	pubRoot := cmd.NewRootCmd()
	pubRoot.SetOut(&bytes.Buffer{})
	pubRoot.SetArgs([]string{
		"publish",
		"--name", "test",
		"--version", "1.0.0",
		"--description", "test",
		"--file", skillFile,
		"--api-url", captureServer.URL,
		"--credentials-path", credsPath,
	})
	if err := pubRoot.Execute(); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if !strings.HasPrefix(gotAuth, "Bearer ") {
		t.Errorf("expected Bearer token in publish request, got %q", gotAuth)
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
