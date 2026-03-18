package auth_test

import (
	"context"
	"slices"
	"testing"

	"github.com/nebari-dev/skillsctl/backend/internal/auth"
)

func TestClaimsContext_RoundTrip(t *testing.T) {
	want := &auth.Claims{
		Subject: "user-123",
		Email:   "user@example.com",
		Groups:  []string{"devs", "admins"},
	}

	ctx := auth.WithClaims(context.Background(), want)
	got, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		t.Fatal("expected claims in context")
	}
	if got.Subject != want.Subject {
		t.Errorf("subject: got %q, want %q", got.Subject, want.Subject)
	}
	if got.Email != want.Email {
		t.Errorf("email: got %q, want %q", got.Email, want.Email)
	}
	if !slices.Equal(got.Groups, want.Groups) {
		t.Errorf("groups: got %v, want %v", got.Groups, want.Groups)
	}
}

func TestClaimsContext_Missing(t *testing.T) {
	_, ok := auth.ClaimsFromContext(context.Background())
	if ok {
		t.Error("expected no claims in empty context")
	}
}

func TestClaimsContext_NilClaims(t *testing.T) {
	ctx := auth.WithClaims(context.Background(), nil)
	_, ok := auth.ClaimsFromContext(ctx)
	if ok {
		t.Error("expected false for nil claims stored in context")
	}
}
