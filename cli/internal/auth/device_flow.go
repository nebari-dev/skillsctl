package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrAuthDisabled is returned when the server has no OIDC configured.
var ErrAuthDisabled = errors.New("server does not require authentication")

// DeviceFlowPending holds the state between StartDeviceFlow and PollForToken.
type DeviceFlowPending struct {
	VerificationURI         string
	VerificationURIComplete string
	UserCode                string
	DeviceCode              string
	TokenEndpoint           string
	ClientID                string
	Interval                time.Duration
	ExpiresAt               time.Time
}

// DeviceFlowResult holds the result of a completed device flow.
type DeviceFlowResult struct {
	IDToken string
	Email   string
	Expiry  time.Time
}

type authConfigResponse struct {
	Enabled   bool   `json:"enabled"`
	IssuerURL string `json:"issuer_url"`
	ClientID  string `json:"client_id"`
}

type oidcDiscovery struct {
	DeviceAuthEndpoint string `json:"device_authorization_endpoint"`
	TokenEndpoint      string `json:"token_endpoint"`
}

type deviceAuthResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type tokenResponse struct {
	IDToken string `json:"id_token"`
	Error   string `json:"error"`
}

// StartDeviceFlow fetches auth config from the skillctl server, discovers
// OIDC endpoints, requests a device code, and returns the pending state
// for display to the user. Call PollForToken after displaying the URL.
func StartDeviceFlow(ctx context.Context, serverURL string) (*DeviceFlowPending, error) {
	authCfg, err := fetchAuthConfig(ctx, serverURL)
	if err != nil {
		return nil, fmt.Errorf("cannot reach server at %s: %w", serverURL, err)
	}
	if !authCfg.Enabled {
		return nil, ErrAuthDisabled
	}

	discovery, err := fetchOIDCDiscovery(ctx, authCfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("cannot reach OIDC provider at %s: %w", authCfg.IssuerURL, err)
	}
	if discovery.DeviceAuthEndpoint == "" {
		return nil, fmt.Errorf("OIDC provider does not support device flow")
	}

	deviceAuth, err := requestDeviceCode(ctx, discovery.DeviceAuthEndpoint, authCfg.ClientID)
	if err != nil {
		return nil, fmt.Errorf("device authorization: %w", err)
	}

	interval := time.Duration(deviceAuth.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}

	return &DeviceFlowPending{
		VerificationURI:         deviceAuth.VerificationURI,
		VerificationURIComplete: deviceAuth.VerificationURIComplete,
		UserCode:                deviceAuth.UserCode,
		DeviceCode:              deviceAuth.DeviceCode,
		TokenEndpoint:           discovery.TokenEndpoint,
		ClientID:                authCfg.ClientID,
		Interval:                interval,
		ExpiresAt:               time.Now().Add(time.Duration(deviceAuth.ExpiresIn) * time.Second),
	}, nil
}

// PollForToken polls the token endpoint until the user completes authentication.
// Pass the pending state from StartDeviceFlow. The pollInterval parameter
// overrides the server-suggested interval (use 0 for the server default).
// In tests, pass a short duration to avoid sleeping.
func PollForToken(ctx context.Context, pending *DeviceFlowPending, pollInterval time.Duration) (*DeviceFlowResult, error) {
	interval := pending.Interval
	if pollInterval > 0 {
		interval = pollInterval
	}

	for {
		if time.Now().After(pending.ExpiresAt) {
			return nil, fmt.Errorf("authentication timed out. Try again")
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}

		tok, err := pollToken(ctx, pending.TokenEndpoint, pending.ClientID, pending.DeviceCode)
		if err != nil {
			return nil, err
		}
		if tok.Error == "authorization_pending" {
			continue
		}
		if tok.Error == "slow_down" {
			interval += 5 * time.Second
			continue
		}
		if tok.Error == "expired_token" {
			return nil, fmt.Errorf("authentication timed out. Try again")
		}
		if tok.Error == "access_denied" {
			return nil, fmt.Errorf("authentication denied")
		}
		if tok.Error != "" {
			return nil, fmt.Errorf("authentication error: %s", tok.Error)
		}

		email, exp := DecodeJWTClaims(tok.IDToken)
		return &DeviceFlowResult{
			IDToken: tok.IDToken,
			Email:   email,
			Expiry:  exp,
		}, nil
	}
}

func fetchAuthConfig(ctx context.Context, serverURL string) (*authConfigResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", serverURL+"/auth/config", nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var cfg authConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func fetchOIDCDiscovery(ctx context.Context, issuerURL string) (*oidcDiscovery, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", issuerURL+"/.well-known/openid-configuration", nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var disc oidcDiscovery
	if err := json.NewDecoder(resp.Body).Decode(&disc); err != nil {
		return nil, err
	}
	return &disc, nil
}

func requestDeviceCode(ctx context.Context, endpoint, clientID string) (*deviceAuthResponse, error) {
	data := url.Values{
		"client_id": {clientID},
		"scope":     {"openid email profile"},
	}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var dar deviceAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&dar); err != nil {
		return nil, err
	}
	return &dar, nil
}

func pollToken(ctx context.Context, endpoint, clientID, deviceCode string) (*tokenResponse, error) {
	data := url.Values{
		"client_id":   {clientID},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var tok tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return nil, err
	}
	return &tok, nil
}

// DecodeJWTClaims decodes the JWT payload without verification.
// Returns email and expiry. This is for display only - the server verifies the token.
func DecodeJWTClaims(idToken string) (email string, expiry time.Time) {
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return "", time.Time{}
	}
	payload := parts[1]
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}
	data, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return "", time.Time{}
	}
	var claims struct {
		Email string `json:"email"`
		Exp   int64  `json:"exp"`
	}
	if err := json.Unmarshal(data, &claims); err != nil {
		return "", time.Time{}
	}
	return claims.Email, time.Unix(claims.Exp, 0)
}
