package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nebari-dev/skillsctl/internal/skillpkg"
)

// GetLimits fetches the server's configured tarball limits from /limits.
func (c *Client) GetLimits(ctx context.Context) (skillpkg.Limits, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/limits", nil)
	if err != nil {
		return skillpkg.Limits{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return skillpkg.Limits{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return skillpkg.Limits{}, fmt.Errorf("limits endpoint returned %d", resp.StatusCode)
	}
	var l skillpkg.Limits
	if err := json.NewDecoder(resp.Body).Decode(&l); err != nil {
		return skillpkg.Limits{}, err
	}
	return l, nil
}
