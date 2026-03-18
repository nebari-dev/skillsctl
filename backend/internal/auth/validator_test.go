package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/nebari-dev/skillsctl/backend/internal/auth"
)

type fakeOIDC struct {
	server *httptest.Server
	key    *rsa.PrivateKey
}

func newFakeOIDC(t *testing.T) *fakeOIDC {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	f := &fakeOIDC{key: key}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", f.handleDiscovery)
	mux.HandleFunc("/jwks", f.handleJWKS)
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeOIDC) handleDiscovery(w http.ResponseWriter, _ *http.Request) {
	doc := map[string]string{
		"issuer":   f.server.URL,
		"jwks_uri": f.server.URL + "/jwks",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(doc)
}

func (f *fakeOIDC) handleJWKS(w http.ResponseWriter, _ *http.Request) {
	jwks := jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Key:       &f.key.PublicKey,
				KeyID:     "test-key",
				Algorithm: string(jose.RS256),
				Use:       "sig",
			},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jwks)
}

type tokenClaims struct {
	Issuer      string
	Audience    []string
	Subject     string
	Email       string
	Groups      []string
	Expiry      time.Time
	ExtraClaims map[string]any
}

func (f *fakeOIDC) mintToken(t *testing.T, tc tokenClaims) string {
	t.Helper()
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: f.key},
		(&jose.SignerOptions{}).WithHeader(jose.HeaderKey("kid"), "test-key"),
	)
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}

	now := time.Now()
	stdClaims := jwt.Claims{
		Issuer:    tc.Issuer,
		Audience:  jwt.Audience(tc.Audience),
		Subject:   tc.Subject,
		IssuedAt:  jwt.NewNumericDate(now),
		Expiry:    jwt.NewNumericDate(tc.Expiry),
		NotBefore: jwt.NewNumericDate(now.Add(-1 * time.Minute)),
	}

	extra := map[string]any{}
	if tc.Email != "" {
		extra["email"] = tc.Email
	}
	if tc.Groups != nil {
		extra["groups"] = tc.Groups
	}
	for k, v := range tc.ExtraClaims {
		extra[k] = v
	}

	raw, err := jwt.Signed(signer).Claims(stdClaims).Claims(extra).Serialize()
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return raw
}

func TestValidator_Validate(t *testing.T) {
	fake := newFakeOIDC(t)
	clientID := "skillsctl-cli"

	cfg := auth.Config{
		IssuerURL:   fake.server.URL,
		ClientID:    clientID,
		AdminGroup:  "skillsctl-admins",
		GroupsClaim: "groups",
	}

	v, err := auth.NewValidator(context.Background(), cfg)
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}

	tests := []struct {
		name       string
		token      tokenClaims
		wantErr    bool
		wantSub    string
		wantEmail  string
		wantGroups []string
	}{
		{
			name: "valid token with all claims",
			token: tokenClaims{
				Issuer:   fake.server.URL,
				Audience: []string{clientID},
				Subject:  "user-123",
				Email:    "user@example.com",
				Groups:   []string{"devs"},
				Expiry:   time.Now().Add(1 * time.Hour),
			},
			wantSub:    "user-123",
			wantEmail:  "user@example.com",
			wantGroups: []string{"devs"},
		},
		{
			name: "expired token",
			token: tokenClaims{
				Issuer:   fake.server.URL,
				Audience: []string{clientID},
				Subject:  "user-123",
				Expiry:   time.Now().Add(-1 * time.Hour),
			},
			wantErr: true,
		},
		{
			name: "wrong audience",
			token: tokenClaims{
				Issuer:   fake.server.URL,
				Audience: []string{"other-client"},
				Subject:  "user-123",
				Expiry:   time.Now().Add(1 * time.Hour),
			},
			wantErr: true,
		},
		{
			name: "missing email claim",
			token: tokenClaims{
				Issuer:   fake.server.URL,
				Audience: []string{clientID},
				Subject:  "user-123",
				Expiry:   time.Now().Add(1 * time.Hour),
			},
			wantSub: "user-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := fake.mintToken(t, tt.token)
			claims, err := v.Validate(context.Background(), raw)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if claims.Subject != tt.wantSub {
				t.Errorf("subject: got %q, want %q", claims.Subject, tt.wantSub)
			}
			if claims.Email != tt.wantEmail {
				t.Errorf("email: got %q, want %q", claims.Email, tt.wantEmail)
			}
			if !slices.Equal(claims.Groups, tt.wantGroups) {
				t.Errorf("groups: got %v, want %v", claims.Groups, tt.wantGroups)
			}
		})
	}
}

func TestValidator_Validate_CustomGroupsClaim(t *testing.T) {
	fake := newFakeOIDC(t)
	clientID := "skillsctl-cli"

	cfg := auth.Config{
		IssuerURL:   fake.server.URL,
		ClientID:    clientID,
		GroupsClaim: "roles",
	}

	v, err := auth.NewValidator(context.Background(), cfg)
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}

	raw := fake.mintToken(t, tokenClaims{
		Issuer:      fake.server.URL,
		Audience:    []string{clientID},
		Subject:     "user-456",
		Expiry:      time.Now().Add(1 * time.Hour),
		ExtraClaims: map[string]any{"roles": []string{"admin", "editor"}},
	})

	claims, err := v.Validate(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantGroups := []string{"admin", "editor"}
	if !slices.Equal(claims.Groups, wantGroups) {
		t.Errorf("groups: got %v, want %v", claims.Groups, wantGroups)
	}
}

func TestIsAdmin(t *testing.T) {
	tests := []struct {
		name   string
		groups []string
		want   bool
	}{
		{"admin member", []string{"devs", "skillsctl-admins"}, true},
		{"not admin", []string{"devs"}, false},
		{"empty groups", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &auth.Claims{Groups: tt.groups}
			if got := auth.IsAdmin("skillsctl-admins", claims); got != tt.want {
				t.Errorf("IsAdmin: got %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("nil claims", func(t *testing.T) {
		if auth.IsAdmin("skillsctl-admins", nil) {
			t.Error("expected false for nil claims")
		}
	})
}
