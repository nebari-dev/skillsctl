package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nebari-dev/skillsctl/cli/internal/api"
	"github.com/nebari-dev/skillsctl/internal/skillpkg"
)

func TestGetLimits(t *testing.T) {
	want := skillpkg.Limits{MaxPackedBytes: 1, MaxTotalBytes: 2, MaxFiles: 3, MaxFileBytes: 4}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/limits" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(ts.Close)

	c := api.NewClient(ts.URL)
	got, err := c.GetLimits(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
}
