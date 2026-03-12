package server_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openteams-ai/skill-share/backend/internal/server"
	"github.com/openteams-ai/skill-share/backend/internal/store"
)

func TestHealthz(t *testing.T) {
	srv := server.New(store.NewMemory(nil))
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Errorf("expected body 'ok', got %q", string(body))
	}
}
