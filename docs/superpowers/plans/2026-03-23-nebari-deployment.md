# Nebari Deployment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deploy skillsctl to a Nebari-managed Hetzner cluster with operator-managed OIDC device flow authentication.

**Architecture:** Three workstreams across repos: (1) nebari-operator gets device flow client provisioning, external issuer URL, Secret RBAC, and ServiceAccount field; (2) skillsctl Helm chart gets multi-mode auth support; (3) ArgoCD app manifest in nic-test repo. Operator and chart work are parallelizable.

**Tech Stack:** Go, Keycloak (gocloak), Kubernetes RBAC, Helm, ArgoCD, ConnectRPC

**Spec:** `docs/superpowers/specs/2026-03-23-nebari-deployment-design.md`

---

## Phase 1: nebari-operator Changes

All work in `/home/chuck/devel/nebari-operator` on a feature branch.

### Task 1: Create feature branch

**Files:** None

- [ ] **Step 1: Create and checkout branch**

```bash
cd /home/chuck/devel/nebari-operator
git checkout main && git pull
git checkout -b feature/device-flow-client
```

- [ ] **Step 2: Verify clean state**

```bash
git status
```

Expected: clean working tree on `feature/device-flow-client`

---

### Task 2: Add DeviceFlowClientConfig type and ServiceAccountName field to CRD

**Files:**
- Modify: `api/v1/nebariapp_types.go:24-57` (NebariAppSpec) and `api/v1/nebariapp_types.go:149-236` (AuthConfig)

- [ ] **Step 1: Write test for new CRD fields**

Create `api/v1/nebariapp_types_test.go`:

```go
package v1

import (
	"encoding/json"
	"testing"
)

func TestDeviceFlowClientConfig_JSON(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected DeviceFlowClientConfig
	}{
		{
			name:     "enabled true",
			json:     `{"enabled": true}`,
			expected: DeviceFlowClientConfig{Enabled: true},
		},
		{
			name:     "enabled false",
			json:     `{"enabled": false}`,
			expected: DeviceFlowClientConfig{Enabled: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got DeviceFlowClientConfig
			if err := json.Unmarshal([]byte(tt.json), &got); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("got %+v, want %+v", got, tt.expected)
			}
		})
	}
}

func TestNebariAppSpec_ServiceAccountName(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected string
	}{
		{
			name:     "omitted defaults to empty",
			json:     `{"hostname":"test.example.com","service":{"name":"svc","port":80}}`,
			expected: "",
		},
		{
			name:     "explicitly set",
			json:     `{"hostname":"test.example.com","service":{"name":"svc","port":80},"serviceAccountName":"my-sa"}`,
			expected: "my-sa",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got NebariAppSpec
			if err := json.Unmarshal([]byte(tt.json), &got); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if got.ServiceAccountName != tt.expected {
				t.Errorf("ServiceAccountName = %q, want %q", got.ServiceAccountName, tt.expected)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/chuck/devel/nebari-operator
go test ./api/v1/... -run TestDeviceFlowClientConfig -v
```

Expected: FAIL - `DeviceFlowClientConfig` not defined

- [ ] **Step 3: Add DeviceFlowClientConfig struct and field to AuthConfig**

In `api/v1/nebariapp_types.go`, after the `SPAClient` field (line 228), add:

```go
	// DeviceFlowClient configures a public OIDC client for CLI/native app authentication
	// using the OAuth2 Device Authorization Grant (RFC 8628).
	// When enabled, the operator provisions a separate public client configured for device flow.
	// The device flow client ID is written to the OIDC client Secret under key "device-client-id".
	// Only supported for provider="keycloak".
	// +optional
	DeviceFlowClient *DeviceFlowClientConfig `json:"deviceFlowClient,omitempty"`
```

After `SPAClientConfig` struct (after line 311), add:

```go
// DeviceFlowClientConfig specifies configuration for provisioning a public OIDC client
// for CLI and native application authentication using the OAuth2 Device Authorization Grant.
type DeviceFlowClientConfig struct {
	// Enabled determines whether a public OIDC client should be provisioned for device flow.
	// When true, the operator creates a Keycloak client configured with:
	//   - publicClient: true (no secret needed for CLI usage)
	//   - OAuth2 Device Authorization Grant enabled
	//   - standardFlowEnabled: false
	// The device flow client ID is written to the OIDC secret for runtime consumption.
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`
}
```

- [ ] **Step 4: Add ServiceAccountName field to NebariAppSpec**

In `api/v1/nebariapp_types.go`, after the `Gateway` field (line 51), add:

```go
	// ServiceAccountName is the name of the Kubernetes ServiceAccount used by the
	// app's pods. Used for RBAC scoping of OIDC secrets so only the app's pods
	// can read its credentials. Defaults to the NebariApp's name if omitted.
	// +optional
	// +kubebuilder:validation:MinLength=1
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
```

- [ ] **Step 5: Regenerate deepcopy and manifests**

```bash
cd /home/chuck/devel/nebari-operator
make generate-dev
```

Expected: `zz_generated.deepcopy.go` updated, CRD manifests regenerated

- [ ] **Step 6: Run tests**

```bash
go test ./api/v1/... -v
```

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add api/ config/crd/
git commit -m "feat: add DeviceFlowClientConfig and ServiceAccountName to NebariApp CRD"
```

---

### Task 3: Add ExternalURL to KeycloakConfig and GetExternalIssuerURL to interface

**Files:**
- Modify: `internal/config/auth.go:40-82` (KeycloakConfig), `internal/config/auth.go:85-103` (LoadAuthConfig)
- Modify: `internal/controller/reconcilers/auth/providers/provider.go:27-49` (OIDCProvider interface)
- Modify: `internal/controller/reconcilers/auth/providers/keycloak.go:40-57` (KeycloakProvider)
- Modify: `internal/controller/reconcilers/auth/providers/generic_oidc.go:33-38` (GenericOIDCProvider)

- [ ] **Step 1: Write tests for GetExternalIssuerURL**

Add to `internal/controller/reconcilers/auth/providers/keycloak_test.go`:

```go
func TestKeycloakProvider_GetExternalIssuerURL(t *testing.T) {
	tests := []struct {
		name        string
		config      config.KeycloakConfig
		nebariApp   *appsv1.NebariApp
		expected    string
		expectError bool
	}{
		{
			name: "External URL configured",
			config: config.KeycloakConfig{
				ExternalURL: "https://keycloak.example.com/auth",
				Realm:       "nebari",
			},
			nebariApp: &appsv1.NebariApp{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			},
			expected: "https://keycloak.example.com/auth/realms/nebari",
		},
		{
			name: "External URL without trailing slash",
			config: config.KeycloakConfig{
				ExternalURL: "https://keycloak.example.com",
				Realm:       "myrealm",
			},
			nebariApp: &appsv1.NebariApp{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			},
			expected: "https://keycloak.example.com/realms/myrealm",
		},
		{
			name: "External URL not configured",
			config: config.KeycloakConfig{
				Realm: "nebari",
			},
			nebariApp: &appsv1.NebariApp{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &KeycloakProvider{Config: tt.config}
			got, err := provider.GetExternalIssuerURL(context.Background(), tt.nebariApp)
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}
```

Add to `internal/controller/reconcilers/auth/providers/generic_oidc_test.go` (create file if needed):

```go
package providers

import (
	"context"
	"testing"

	appsv1 "github.com/nebari-dev/nebari-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenericOIDCProvider_GetExternalIssuerURL(t *testing.T) {
	provider := &GenericOIDCProvider{}
	app := &appsv1.NebariApp{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: appsv1.NebariAppSpec{
			Auth: &appsv1.AuthConfig{
				IssuerURL: "https://accounts.google.com",
			},
		},
	}

	got, err := provider.GetExternalIssuerURL(context.Background(), app)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://accounts.google.com" {
		t.Errorf("got %q, want %q", got, "https://accounts.google.com")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/controller/reconcilers/auth/providers/... -run TestKeycloakProvider_GetExternalIssuerURL -v
```

Expected: FAIL - `GetExternalIssuerURL` not defined

- [ ] **Step 3: Add ExternalURL to KeycloakConfig**

In `internal/config/auth.go`, add field to `KeycloakConfig` struct (after `IssuerContextPath` at line 77):

```go
	// ExternalURL is the publicly routable URL for Keycloak.
	// Used to construct the issuer URL written to OIDC client Secrets for external consumers
	// (CLI tools, /auth/config endpoints). Must be reachable from outside the cluster.
	// Example: https://keycloak.example.com/auth
	ExternalURL string
```

In `LoadAuthConfig()`, add to the KeycloakConfig initialization (after `IssuerContextPath` line 99):

```go
			ExternalURL:            getEnv("KEYCLOAK_EXTERNAL_URL", ""),
```

- [ ] **Step 4: Add GetExternalIssuerURL to OIDCProvider interface**

In `internal/controller/reconcilers/auth/providers/provider.go`, add to the interface (after `GetIssuerURL`):

```go
	// GetExternalIssuerURL returns the publicly routable OIDC issuer URL.
	// This URL is written to the client Secret for external consumers (CLIs, frontends).
	// For Keycloak, this uses the externally-facing URL configured via KEYCLOAK_EXTERNAL_URL.
	// For generic OIDC, this returns the same value as GetIssuerURL (already external).
	GetExternalIssuerURL(ctx context.Context, nebariApp *appsv1.NebariApp) (string, error)
```

- [ ] **Step 5: Implement GetExternalIssuerURL on KeycloakProvider**

In `internal/controller/reconcilers/auth/providers/keycloak.go`, after `GetIssuerURL` (after line 57):

```go
// GetExternalIssuerURL returns the publicly routable Keycloak issuer URL.
// Uses KEYCLOAK_EXTERNAL_URL to construct a URL reachable from outside the cluster.
func (p *KeycloakProvider) GetExternalIssuerURL(ctx context.Context, nebariApp *appsv1.NebariApp) (string, error) {
	if p.Config.ExternalURL == "" {
		return "", fmt.Errorf("KEYCLOAK_EXTERNAL_URL not configured; required for external issuer URL")
	}
	return fmt.Sprintf("%s/realms/%s", strings.TrimRight(p.Config.ExternalURL, "/"), p.Config.Realm), nil
}
```

Add `"strings"` to imports.

- [ ] **Step 6: Implement GetExternalIssuerURL on GenericOIDCProvider**

In `internal/controller/reconcilers/auth/providers/generic_oidc.go`, after `GetIssuerURL`:

```go
// GetExternalIssuerURL returns the same URL as GetIssuerURL for generic OIDC.
// Generic OIDC issuer URLs are already externally routable by definition.
func (p *GenericOIDCProvider) GetExternalIssuerURL(ctx context.Context, nebariApp *appsv1.NebariApp) (string, error) {
	return p.GetIssuerURL(ctx, nebariApp)
}
```

- [ ] **Step 7: Run tests**

```bash
go test ./internal/... -run "TestKeycloakProvider_GetExternalIssuerURL|TestGenericOIDCProvider_GetExternalIssuerURL" -v
```

Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/ api/
git commit -m "feat: add GetExternalIssuerURL to OIDCProvider interface and KEYCLOAK_EXTERNAL_URL config"
```

---

### Task 4: Add constants and naming helpers for device flow

**Files:**
- Modify: `internal/controller/utils/constants/constants.go:91-99`
- Modify: `internal/controller/utils/naming/naming.go:74-79`

- [ ] **Step 1: Add constants**

In `internal/controller/utils/constants/constants.go`, add to "Secret keys" block (after line 98):

```go
	// ClientIDKey is the key name for OIDC client ID data
	ClientIDKey = "client-id"

	// DeviceClientIDKey is the key name for device flow client ID data
	DeviceClientIDKey = "device-client-id"

	// IssuerURLKey is the key name for the external OIDC issuer URL
	IssuerURLKey = "issuer-url"
```

- [ ] **Step 2: Add DeviceFlowClientID naming function**

In `internal/controller/utils/naming/naming.go`, after `ClientID` function (after line 79):

```go
// DeviceFlowClientID generates the OIDC device flow client ID for a NebariApp.
// Pattern: <namespace>-<nebariapp-name>-device
func DeviceFlowClientID(nebariApp *appsv1.NebariApp) string {
	return fmt.Sprintf("%s-%s-device", nebariApp.Namespace, nebariApp.Name)
}
```

- [ ] **Step 3: Run existing tests to verify no regressions**

```bash
go test ./internal/controller/utils/... -v
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/controller/utils/
git commit -m "feat: add device flow constants and naming helper"
```

---

### Task 5: Implement device flow client provisioning

**Files:**
- Modify: `internal/controller/reconcilers/auth/providers/keycloak.go:140-209` (ProvisionClient), `keycloak.go:372-412` (storeClientSecret), `keycloak.go:210-262` (DeleteClient)

- [ ] **Step 1: Write test for storeClientSecret with new fields**

Add test case to `TestKeycloakProvider_StoreClientSecret` in `keycloak_test.go`:

```go
		{
			name: "Create secret with all fields",
			nebariApp: &appsv1.NebariApp{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app",
					Namespace: "default",
					UID:       "test-uid",
				},
			},
			clientSecret:    "test-secret-value",
			clientID:        "default-test-app",
			externalIssuer:  "https://keycloak.example.com/auth/realms/nebari",
			deviceClientID:  "default-test-app-device",
			existingSecret:  nil,
			expectError:     false,
		},
```

Note: The test struct and assertions need updating to match the new `storeClientSecret` signature. Update the test struct to include `clientID`, `externalIssuer`, and `deviceClientID` string fields. Update the call site to pass all parameters. Add assertions to verify all keys are present in the created Secret.

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/controller/reconcilers/auth/providers/... -run TestKeycloakProvider_StoreClientSecret -v
```

Expected: FAIL - signature mismatch

- [ ] **Step 3: Update storeClientSecret signature and implementation**

In `keycloak.go`, replace the `storeClientSecret` method:

```go
// storeClientSecret creates or updates the Kubernetes secret containing OIDC client credentials.
// Stores: client-id, client-secret, issuer-url (always), plus device-client-id and spa-client-id (conditionally).
func (p *KeycloakProvider) storeClientSecret(ctx context.Context, nebariApp *appsv1.NebariApp, clientID, clientSecret, externalIssuerURL, spaClientID, deviceClientID string) error {
	secretName := naming.ClientSecretName(nebariApp)

	secretData := map[string][]byte{
		constants.ClientIDKey:     []byte(clientID),
		constants.ClientSecretKey: []byte(clientSecret),
		constants.IssuerURLKey:    []byte(externalIssuerURL),
	}

	if spaClientID != "" {
		secretData[constants.SPAClientIDKey] = []byte(spaClientID)
	}

	if deviceClientID != "" {
		secretData[constants.DeviceClientIDKey] = []byte(deviceClientID)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: nebariApp.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "nebariapp",
				"app.kubernetes.io/instance":   nebariApp.Name,
				"app.kubernetes.io/managed-by": "nebari-operator",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: secretData,
	}

	existingSecret := &corev1.Secret{}
	err := p.Client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: nebariApp.Namespace}, existingSecret)

	if apierrors.IsNotFound(err) {
		return p.Client.Create(ctx, secret)
	} else if err != nil {
		return fmt.Errorf("failed to check for existing secret: %w", err)
	}

	existingSecret.Data = secret.Data
	return p.Client.Update(ctx, existingSecret)
}
```

- [ ] **Step 4: Add shouldProvisionDeviceFlowClient and provisionDeviceFlowClient**

In `keycloak.go`, after the `provisionSPAClient` method (after line 817):

```go
// shouldProvisionDeviceFlowClient returns true if the operator should provision a device flow client.
func (p *KeycloakProvider) shouldProvisionDeviceFlowClient(nebariApp *appsv1.NebariApp) bool {
	return nebariApp.Spec.Auth != nil &&
		nebariApp.Spec.Auth.DeviceFlowClient != nil &&
		nebariApp.Spec.Auth.DeviceFlowClient.Enabled
}

// GetDeviceFlowClientID returns the device flow client ID for the NebariApp.
func (p *KeycloakProvider) GetDeviceFlowClientID(ctx context.Context, nebariApp *appsv1.NebariApp) string {
	return naming.DeviceFlowClientID(nebariApp)
}

// provisionDeviceFlowClient creates or updates a public OIDC client for device flow authentication.
// The client is configured with OAuth2 Device Authorization Grant enabled and an audience mapper
// that adds the confidential client's ID to the aud claim.
func (p *KeycloakProvider) provisionDeviceFlowClient(ctx context.Context, kcClient *gocloak.GoCloak, token *gocloak.JWT, nebariApp *appsv1.NebariApp) (string, error) {
	logger := log.FromContext(ctx)
	deviceClientID := p.GetDeviceFlowClientID(ctx, nebariApp)
	confidentialClientID := p.GetClientID(ctx, nebariApp)

	existingClient, err := p.findClient(ctx, kcClient, token, deviceClientID)
	if err != nil {
		return "", fmt.Errorf("failed to query for device flow client: %w", err)
	}

	// Device flow clients don't need redirect URIs
	attrs := map[string]string{
		"oauth2.device.authorization.grant.enabled": "true",
	}

	if existingClient != nil {
		existingClient.PublicClient = gocloak.BoolP(true)
		existingClient.StandardFlowEnabled = gocloak.BoolP(false)
		existingClient.DirectAccessGrantsEnabled = gocloak.BoolP(false)
		if existingClient.Attributes == nil {
			existingClient.Attributes = &map[string]string{}
		}
		for k, v := range attrs {
			(*existingClient.Attributes)[k] = v
		}

		err = kcClient.UpdateClient(ctx, token.AccessToken, p.Config.Realm, *existingClient)
		if err != nil {
			return "", fmt.Errorf("failed to update device flow client: %w", err)
		}
		logger.Info("Updated existing device flow client", "clientID", deviceClientID)
	} else {
		newClient := gocloak.Client{
			ClientID:                  gocloak.StringP(deviceClientID),
			Name:                      gocloak.StringP(fmt.Sprintf("%s Device Flow Client", nebariApp.Name)),
			PublicClient:              gocloak.BoolP(true),
			StandardFlowEnabled:       gocloak.BoolP(false),
			DirectAccessGrantsEnabled: gocloak.BoolP(false),
			Protocol:                  gocloak.StringP("openid-connect"),
			Enabled:                   gocloak.BoolP(true),
			Attributes:                &attrs,
		}

		_, err := kcClient.CreateClient(ctx, token.AccessToken, p.Config.Realm, newClient)
		if err != nil {
			return "", fmt.Errorf("failed to create device flow client: %w", err)
		}
		logger.Info("Created new device flow client", "clientID", deviceClientID)
	}

	// Sync scopes from spec (same scopes as confidential client)
	deviceClient, err := p.findClient(ctx, kcClient, token, deviceClientID)
	if err != nil || deviceClient == nil {
		return "", fmt.Errorf("failed to find device flow client after creation: %w", err)
	}
	if err := p.syncClientScopes(ctx, kcClient, token, *deviceClient.ID, nebariApp); err != nil {
		return "", fmt.Errorf("failed to sync device flow client scopes: %w", err)
	}

	// Add audience mapper so device flow tokens include the confidential client ID in aud claim
	audienceMapper := gocloak.ProtocolMapperRepresentation{
		Name:           gocloak.StringP("audience-confidential-client"),
		Protocol:       gocloak.StringP("openid-connect"),
		ProtocolMapper: gocloak.StringP("oidc-audience-mapper"),
		Config: &map[string]string{
			"included.client.audience": confidentialClientID,
			"id.token.claim":           "true",
			"access.token.claim":       "true",
		},
	}

	// Fetch existing protocol mappers via the dedicated endpoint.
	// NOTE: GetClient does NOT return ProtocolMappers - must use GetClientProtocolMappers.
	existingMappers, err := kcClient.GetClientProtocolMappers(ctx, token.AccessToken, p.Config.Realm, *deviceClient.ID)
	if err != nil {
		return "", fmt.Errorf("failed to get device flow client protocol mappers: %w", err)
	}

	mapperExists := false
	for _, m := range existingMappers {
		if m.Name != nil && *m.Name == "audience-confidential-client" {
			mapperExists = true
			audienceMapper.ID = m.ID
			err = kcClient.UpdateClientProtocolMapper(ctx, token.AccessToken, p.Config.Realm, *deviceClient.ID, *m.ID, audienceMapper)
			if err != nil {
				return "", fmt.Errorf("failed to update audience mapper: %w", err)
			}
			break
		}
	}

	if !mapperExists {
		_, err = kcClient.CreateClientProtocolMapper(ctx, token.AccessToken, p.Config.Realm, *deviceClient.ID, audienceMapper)
		if err != nil {
			return "", fmt.Errorf("failed to create audience mapper: %w", err)
		}
	}

	logger.Info("Configured audience mapper on device flow client",
		"deviceClientID", deviceClientID,
		"audienceClientID", confidentialClientID)

	return deviceClientID, nil
}
```

- [ ] **Step 5: Update ProvisionClient to call device flow provisioning and pass new args to storeClientSecret**

Replace the end of `ProvisionClient` (lines 197-208) with:

```go
	// Provision SPA client if requested
	var spaClientID string
	if p.shouldProvisionSPAClient(nebariApp) {
		spaClientID, err = p.provisionSPAClient(ctx, kcClient, token, nebariApp)
		if err != nil {
			return fmt.Errorf("failed to provision SPA client: %w", err)
		}
		logger.Info("Provisioned SPA client", "clientID", spaClientID)
	}

	// Provision device flow client if requested
	var deviceClientID string
	if p.shouldProvisionDeviceFlowClient(nebariApp) {
		deviceClientID, err = p.provisionDeviceFlowClient(ctx, kcClient, token, nebariApp)
		if err != nil {
			return fmt.Errorf("failed to provision device flow client: %w", err)
		}
		logger.Info("Provisioned device flow client", "clientID", deviceClientID)
	}

	// Get external issuer URL for the Secret
	externalIssuerURL, err := p.GetExternalIssuerURL(ctx, nebariApp)
	if err != nil {
		return fmt.Errorf("failed to get external issuer URL: %w", err)
	}

	// Store all credentials in Kubernetes Secret
	return p.storeClientSecret(ctx, nebariApp, clientID, clientSecret, externalIssuerURL, spaClientID, deviceClientID)
```

- [ ] **Step 6: Update DeleteClient to also delete device flow client**

In `DeleteClient`, after the SPA client deletion block (after line 259), add:

```go
	// Delete device flow client if it exists
	if p.shouldProvisionDeviceFlowClient(nebariApp) {
		deviceClientID := p.GetDeviceFlowClientID(ctx, nebariApp)
		existingDeviceClient, err := p.findClient(ctx, kcClient, token, deviceClientID)
		if err != nil {
			return err
		}

		if existingDeviceClient != nil {
			err = kcClient.DeleteClient(ctx, token.AccessToken, p.Config.Realm, *existingDeviceClient.ID)
			if err != nil {
				return fmt.Errorf("failed to delete device flow client: %w", err)
			}
			logger.Info("Deleted device flow client", "clientID", deviceClientID)
		}
	}
```

- [ ] **Step 7: Update existing tests for new storeClientSecret signature**

Update all calls to `storeClientSecret` in `keycloak_test.go` to match the new signature: `storeClientSecret(ctx, nebariApp, clientID, clientSecret, externalIssuerURL, spaClientID, deviceClientID)`.

- [ ] **Step 8: Run tests**

```bash
go test ./internal/controller/reconcilers/auth/providers/... -v
```

Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add internal/
git commit -m "feat: implement device flow client provisioning with audience mapper"
```

---

### Task 6: Implement Secret RBAC (Role + RoleBinding)

**Files:**
- Modify: `internal/controller/reconcilers/auth/reconciler.go:72-145` (ReconcileAuth)
- Modify: `internal/controller/nebariapp_controller.go:60-70` (RBAC markers)

- [ ] **Step 1: Write test for RBAC reconciliation**

Create `internal/controller/reconcilers/auth/rbac_test.go`:

```go
package auth

import (
	"context"
	"testing"

	appsv1 "github.com/nebari-dev/nebari-operator/api/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileSecretRBAC(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)

	tests := []struct {
		name               string
		nebariApp          *appsv1.NebariApp
		expectedSAName     string
	}{
		{
			name: "Default ServiceAccount name",
			nebariApp: &appsv1.NebariApp{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-app",
					Namespace: "default",
					UID:       "test-uid",
				},
				Spec: appsv1.NebariAppSpec{
					Hostname: "my-app.example.com",
					Service:  appsv1.ServiceReference{Name: "svc", Port: 80},
					Auth:     &appsv1.AuthConfig{Enabled: true},
				},
			},
			expectedSAName: "my-app",
		},
		{
			name: "Custom ServiceAccount name",
			nebariApp: &appsv1.NebariApp{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-app",
					Namespace: "default",
					UID:       "test-uid",
				},
				Spec: appsv1.NebariAppSpec{
					Hostname:           "my-app.example.com",
					Service:            appsv1.ServiceReference{Name: "svc", Port: 80},
					ServiceAccountName: "custom-sa",
					Auth:               &appsv1.AuthConfig{Enabled: true},
				},
			},
			expectedSAName: "custom-sa",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.nebariApp).
				Build()

			r := &AuthReconciler{Client: client}
			err := r.reconcileSecretRBAC(context.Background(), tt.nebariApp)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify Role
			role := &rbacv1.Role{}
			err = client.Get(context.Background(), types.NamespacedName{
				Name:      tt.nebariApp.Name + "-oidc-secret-reader",
				Namespace: tt.nebariApp.Namespace,
			}, role)
			if err != nil {
				t.Fatalf("failed to get Role: %v", err)
			}
			if len(role.Rules) != 1 || role.Rules[0].ResourceNames[0] != tt.nebariApp.Name+"-oidc-client" {
				t.Errorf("unexpected Role rules: %+v", role.Rules)
			}

			// Verify RoleBinding
			rb := &rbacv1.RoleBinding{}
			err = client.Get(context.Background(), types.NamespacedName{
				Name:      tt.nebariApp.Name + "-oidc-secret-reader",
				Namespace: tt.nebariApp.Namespace,
			}, rb)
			if err != nil {
				t.Fatalf("failed to get RoleBinding: %v", err)
			}
			if rb.Subjects[0].Name != tt.expectedSAName {
				t.Errorf("RoleBinding subject = %q, want %q", rb.Subjects[0].Name, tt.expectedSAName)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/controller/reconcilers/auth/... -run TestReconcileSecretRBAC -v
```

Expected: FAIL - `reconcileSecretRBAC` not defined

- [ ] **Step 3: Implement reconcileSecretRBAC**

Create `internal/controller/reconcilers/auth/rbac.go`:

```go
package auth

import (
	"context"
	"fmt"

	appsv1 "github.com/nebari-dev/nebari-operator/api/v1"
	"github.com/nebari-dev/nebari-operator/internal/controller/utils/naming"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// reconcileSecretRBAC creates or updates a Role and RoleBinding that scopes
// read access to the OIDC client Secret to the app's ServiceAccount.
func (r *AuthReconciler) reconcileSecretRBAC(ctx context.Context, nebariApp *appsv1.NebariApp) error {
	logger := log.FromContext(ctx)

	secretName := naming.ClientSecretName(nebariApp)
	rbacName := fmt.Sprintf("%s-oidc-secret-reader", nebariApp.Name)

	// Determine ServiceAccount name
	saName := nebariApp.Spec.ServiceAccountName
	if saName == "" {
		saName = nebariApp.Name
	}

	// Reconcile Role
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbacName,
			Namespace: nebariApp.Namespace,
		},
	}

	existing := &rbacv1.Role{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: rbacName, Namespace: nebariApp.Namespace}, existing)
	if apierrors.IsNotFound(err) {
		role.Rules = []rbacv1.PolicyRule{
			{
				APIGroups:     []string{""},
				Resources:     []string{"secrets"},
				ResourceNames: []string{secretName},
				Verbs:         []string{"get"},
			},
		}
		if err := controllerutil.SetControllerReference(nebariApp, role, r.Client.Scheme()); err != nil {
			return fmt.Errorf("failed to set owner reference on Role: %w", err)
		}
		if err := r.Client.Create(ctx, role); err != nil {
			return fmt.Errorf("failed to create Role: %w", err)
		}
		logger.Info("Created OIDC secret reader Role", "name", rbacName, "serviceAccount", saName)
	} else if err != nil {
		return fmt.Errorf("failed to get Role: %w", err)
	} else {
		existing.Rules = []rbacv1.PolicyRule{
			{
				APIGroups:     []string{""},
				Resources:     []string{"secrets"},
				ResourceNames: []string{secretName},
				Verbs:         []string{"get"},
			},
		}
		if err := r.Client.Update(ctx, existing); err != nil {
			return fmt.Errorf("failed to update Role: %w", err)
		}
	}

	// Reconcile RoleBinding
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbacName,
			Namespace: nebariApp.Namespace,
		},
	}

	existingRB := &rbacv1.RoleBinding{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: rbacName, Namespace: nebariApp.Namespace}, existingRB)
	if apierrors.IsNotFound(err) {
		rb.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     rbacName,
		}
		rb.Subjects = []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: nebariApp.Namespace,
			},
		}
		if err := controllerutil.SetControllerReference(nebariApp, rb, r.Client.Scheme()); err != nil {
			return fmt.Errorf("failed to set owner reference on RoleBinding: %w", err)
		}
		if err := r.Client.Create(ctx, rb); err != nil {
			return fmt.Errorf("failed to create RoleBinding: %w", err)
		}
		logger.Info("Created OIDC secret reader RoleBinding", "name", rbacName, "serviceAccount", saName)
	} else if err != nil {
		return fmt.Errorf("failed to get RoleBinding: %w", err)
	} else {
		existingRB.Subjects = []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: nebariApp.Namespace,
			},
		}
		if err := r.Client.Update(ctx, existingRB); err != nil {
			return fmt.Errorf("failed to update RoleBinding: %w", err)
		}
	}

	return nil
}
```

- [ ] **Step 4: Wire reconcileSecretRBAC into ReconcileAuth**

In `reconciler.go`, inside the `shouldProvisionClient` block, after `r.Recorder.Event(...)` (the "OIDC client provisioned successfully" line) and before the `validateAuthConfig` call, add:

```go
		// Reconcile RBAC for OIDC secret access (only when operator manages the Secret)
		logger.Info("Reconciling Secret RBAC")
		if err := r.reconcileSecretRBAC(ctx, nebariApp); err != nil {
			conditions.SetCondition(nebariApp, appsv1.ConditionTypeAuthReady, metav1.ConditionFalse,
				"RBACFailed", fmt.Sprintf("Failed to reconcile Secret RBAC: %v", err))
			return err
		}
```

Note: This must be inside the `if shouldProvisionClient` block. If the operator doesn't manage the Secret (provisionClient: false), RBAC shouldn't be created for it.

- [ ] **Step 5: Add RBAC markers for Roles and RoleBindings**

In `internal/controller/nebariapp_controller.go`, add after line 65:

```go
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
```

- [ ] **Step 6: Regenerate manifests**

```bash
cd /home/chuck/devel/nebari-operator
make manifests
```

- [ ] **Step 7: Run all tests**

```bash
go test ./internal/controller/reconcilers/auth/... -v
```

Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/ config/
git commit -m "feat: add RBAC for OIDC Secret scoped to app ServiceAccount"
```

---

### Task 7: Run full test suite and lint

**Files:** None

- [ ] **Step 1: Run all unit tests**

```bash
cd /home/chuck/devel/nebari-operator
make test-unit
```

Expected: PASS

- [ ] **Step 2: Run linter**

```bash
make lint
```

Expected: PASS (fix any issues)

- [ ] **Step 3: Verify generated code is up to date**

```bash
make generate-dev
git diff --exit-code
```

Expected: No changes (generated code is committed)

- [ ] **Step 4: Commit any fixes**

Only if lint or test fixes were needed.

---

### Task 8: Open PR for operator changes

**Files:** None

- [ ] **Step 1: Push branch**

```bash
cd /home/chuck/devel/nebari-operator
git push -u origin feature/device-flow-client
```

- [ ] **Step 2: Create PR**

```bash
gh pr create \
  --title "Add device flow client provisioning, Secret RBAC, and ServiceAccount field" \
  --body "$(cat <<'EOF'
## Summary

Closes #83

- Add `DeviceFlowClientConfig` to `AuthConfig` for provisioning public OAuth2 device flow clients
- Add `ServiceAccountName` to `NebariAppSpec` for RBAC targeting (defaults to NebariApp name)
- Add `GetExternalIssuerURL()` to `OIDCProvider` interface, backed by `KEYCLOAK_EXTERNAL_URL` env var
- Provision device flow Keycloak client with audience mapper pointing to confidential client
- Write `client-id`, `issuer-url`, `device-client-id` to OIDC Secret
- Create Role + RoleBinding scoping Secret read access to the app's ServiceAccount
- Clean up device flow client on NebariApp deletion

## Test plan

- [ ] Unit tests pass (`make test-unit`)
- [ ] Lint passes (`make lint`)
- [ ] Generated code is up to date (`make generate-dev && git diff --exit-code`)
- [ ] Deploy to Hetzner cluster and verify device flow client appears in Keycloak
- [ ] Verify Secret contains all expected keys
- [ ] Verify RBAC Role/RoleBinding are created
EOF
)"
```

---

## Phase 2: skillsctl Chart and Backend Changes

All work in `/home/chuck/devel/skillctl` on a feature branch.

### Task 9: Create feature branch for chart changes

**Files:** None

- [ ] **Step 1: Create and checkout branch**

```bash
cd /home/chuck/devel/skillctl
git checkout main && git pull
git checkout -b feature/nebari-auth-modes
```

---

### Task 10: Add deviceClientID to values.yaml and update ConfigMap template

**NOTE:** Line numbers in Phase 2 tasks are approximate and may drift after insertions. Always read the current file state before editing. Use `grep -n` to find exact insertion points.

**Files:**
- Modify: `chart/values.yaml` (oidc section, nebariapp section)
- Modify: `chart/templates/configmap.yaml`

- [ ] **Step 1: Add deviceClientID to values.yaml**

In `chart/values.yaml`, after `clientID: ""` (line 39), add:

```yaml
  deviceClientID: ""  # Public client ID for CLI device flow auth (standalone mode only)
```

Add `serviceAccountName` to nebariapp section, after `enabled: false` (line 79):

```yaml
  serviceAccountName: ""  # Passed to NebariApp CRD, defaults to release fullname
```

Add `deviceFlowClient` to nebariapp.auth section, after `provisionClient: true`:

```yaml
    deviceFlowClient:
      enabled: false
```

- [ ] **Step 2: Update ConfigMap to gate OIDC values on nebariapp mode**

Replace `chart/templates/configmap.yaml` content:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ include "skillsctl.fullname" . }}
  labels:
    {{- include "skillsctl.labels" . | nindent 4 }}
data:
  PORT: {{ .Values.server.port | quote }}
  DB_PATH: {{ .Values.server.dbPath | quote }}
  {{- if .Values.server.devMode }}
  DEV_MODE: "true"
  {{- end }}
  {{- if not .Values.nebariapp.enabled }}
  {{- if .Values.oidc.issuerURL }}
  OIDC_ISSUER_URL: {{ .Values.oidc.issuerURL | quote }}
  {{- end }}
  {{- if .Values.oidc.clientID }}
  OIDC_CLIENT_ID: {{ .Values.oidc.clientID | quote }}
  {{- end }}
  {{- if .Values.oidc.deviceClientID }}
  OIDC_DEVICE_CLIENT_ID: {{ .Values.oidc.deviceClientID | quote }}
  {{- end }}
  {{- end }}
  {{- if .Values.oidc.adminGroup }}
  OIDC_ADMIN_GROUP: {{ .Values.oidc.adminGroup | quote }}
  {{- end }}
  {{- if .Values.oidc.groupsClaim }}
  OIDC_GROUPS_CLAIM: {{ .Values.oidc.groupsClaim | quote }}
  {{- end }}
```

- [ ] **Step 3: Verify with helm template**

```bash
cd /home/chuck/devel/skillctl
helm template test chart/ --set oidc.issuerURL=https://example.com --set nebariapp.enabled=true | grep OIDC
```

Expected: Only `OIDC_ADMIN_GROUP` and `OIDC_GROUPS_CLAIM` appear (no `OIDC_ISSUER_URL`)

```bash
helm template test chart/ --set oidc.issuerURL=https://example.com | grep OIDC
```

Expected: `OIDC_ISSUER_URL`, `OIDC_ADMIN_GROUP`, `OIDC_GROUPS_CLAIM` all appear

- [ ] **Step 4: Commit**

```bash
git add chart/values.yaml chart/templates/configmap.yaml
git commit -m "feat: gate OIDC ConfigMap values on nebariapp mode, add deviceClientID"
```

---

### Task 11: Update Deployment to inject Secret env vars in nebari mode

**Files:**
- Modify: `chart/templates/deployment.yaml:34-52`

- [ ] **Step 1: Add Secret env vars block to deployment**

In `chart/templates/deployment.yaml`, after the `envFrom` block (after line 52), add:

```yaml
          {{- if .Values.nebariapp.enabled }}
          env:
            - name: OIDC_ISSUER_URL
              valueFrom:
                secretKeyRef:
                  name: {{ include "skillsctl.fullname" . }}-oidc-client
                  key: issuer-url
            - name: OIDC_CLIENT_ID
              valueFrom:
                secretKeyRef:
                  name: {{ include "skillsctl.fullname" . }}-oidc-client
                  key: client-id
            - name: OIDC_DEVICE_CLIENT_ID
              valueFrom:
                secretKeyRef:
                  name: {{ include "skillsctl.fullname" . }}-oidc-client
                  key: device-client-id
                  optional: true
          {{- end }}
```

- [ ] **Step 2: Verify with helm template**

```bash
helm template test chart/ --set nebariapp.enabled=true --set nebariapp.hostname=test.example.com | grep -A5 secretKeyRef
```

Expected: Three secretKeyRef blocks for issuer-url, client-id, device-client-id

```bash
helm template test chart/ | grep secretKeyRef
```

Expected: No secretKeyRef (standalone mode)

- [ ] **Step 3: Commit**

```bash
git add chart/templates/deployment.yaml
git commit -m "feat: inject OIDC credentials from operator Secret in nebari mode"
```

---

### Task 12: Update NebariApp template with new fields

**Files:**
- Modify: `chart/templates/nebariapp.yaml`

- [ ] **Step 1: Add serviceAccountName and deviceFlowClient to template**

Update `chart/templates/nebariapp.yaml` to include the new fields. After `hostname:` line, add:

```yaml
  serviceAccountName: {{ .Values.nebariapp.serviceAccountName | default (include "skillsctl.fullname" .) }}
```

Inside the auth block (which is inside `{{- with .Values.nebariapp.auth }}`), after `provisionClient:`, add using **relative context** (the `with` block changes `.` to `.Values.nebariapp.auth`):

```yaml
    {{- if .deviceFlowClient }}
    deviceFlowClient:
      enabled: {{ .deviceFlowClient.enabled }}
    {{- end }}
```

IMPORTANT: Inside a `{{- with }}` block, use `.fieldName` (relative), not `.Values.nebariapp.auth.fieldName` (absolute). Absolute references inside `with` require the `$` root prefix (`$.Values...`).

- [ ] **Step 2: Verify with helm template**

```bash
helm template test chart/ --set nebariapp.enabled=true --set nebariapp.hostname=test.example.com --set nebariapp.auth.deviceFlowClient.enabled=true | grep -A2 deviceFlowClient
```

Expected: `deviceFlowClient:` with `enabled: true`

- [ ] **Step 3: Commit**

```bash
git add chart/templates/nebariapp.yaml
git commit -m "feat: add serviceAccountName and deviceFlowClient to NebariApp template"
```

---

### Task 13: Add device_client_id to /auth/config endpoint

**Files:**
- Modify: `backend/internal/auth/` (auth config struct)
- Modify: `backend/cmd/server/main.go` (env var reading)
- Modify: server handler for /auth/config

- [ ] **Step 1: Find the auth config and handler files**

```bash
grep -rn "auth/config" /home/chuck/devel/skillctl/backend/ --include="*.go" | head -20
grep -rn "OIDC_CLIENT_ID" /home/chuck/devel/skillctl/backend/ --include="*.go" | head -10
grep -rn "device" /home/chuck/devel/skillctl/backend/ --include="*.go" | head -10
```

Use the results to identify exact files and line numbers. The implementation must:
1. Add `DeviceClientID string` to the auth config struct
2. Read `OIDC_DEVICE_CLIENT_ID` env var in main.go
3. Include `device_client_id` in the /auth/config JSON response (omit if empty)

- [ ] **Step 2: Write test for /auth/config with device_client_id**

Write a table-driven test verifying:
- When DeviceClientID is set, response includes `device_client_id`
- When DeviceClientID is empty, response omits `device_client_id`

- [ ] **Step 3: Run test to verify it fails**

- [ ] **Step 4: Implement the changes**

- [ ] **Step 5: Run tests**

```bash
go test ./backend/... -race -v
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/
git commit -m "feat: add device_client_id to /auth/config endpoint"
```

---

### Task 14: Add CI test values for all chart modes

**Files:**
- Modify: `chart/ci/test-values.yaml`
- Create: `chart/ci/nebari-values.yaml`
- Create: `chart/ci/devmode-values.yaml`

- [ ] **Step 1: Update existing test-values.yaml**

Add `oidc.deviceClientID` to the existing standalone test values.

- [ ] **Step 2: Create nebari-values.yaml**

```yaml
image:
  tag: latest

persistence:
  enabled: false

nebariapp:
  enabled: true
  hostname: skillsctl.test.example.com
  auth:
    enabled: true
    provider: keycloak
    provisionClient: true
    deviceFlowClient:
      enabled: true
    scopes:
      - openid
      - profile
      - email
```

- [ ] **Step 3: Create devmode-values.yaml**

```yaml
image:
  tag: latest

persistence:
  enabled: false

server:
  devMode: true
```

- [ ] **Step 4: Verify all three render without errors**

```bash
for f in chart/ci/*.yaml; do echo "=== $f ===" && helm template test chart/ -f "$f" > /dev/null && echo "OK"; done
```

Expected: All three print OK

- [ ] **Step 5: Commit**

```bash
git add chart/ci/
git commit -m "feat: add CI test values for nebari and devmode chart configurations"
```

---

### Task 15: Run full test suite, lint, and open PR

**Files:** None

- [ ] **Step 1: Run all tests**

```bash
cd /home/chuck/devel/skillctl
make test
make lint
```

Expected: PASS

- [ ] **Step 2: Push and open PR**

```bash
git push -u origin feature/nebari-auth-modes
gh pr create \
  --title "Add multi-mode OIDC auth support to Helm chart" \
  --body "$(cat <<'EOF'
## Summary

- Gate OIDC ConfigMap values behind nebariapp mode check
- Inject OIDC credentials from operator-created Secret in nebari mode
- Add serviceAccountName and deviceFlowClient to NebariApp template
- Add device_client_id to /auth/config backend endpoint
- Add CI test values for standalone, nebari, and devmode configurations

## Test plan

- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] `helm template` renders correctly for all three modes
- [ ] Deploy to Hetzner cluster with ArgoCD
EOF
)"
```

---

## Phase 3: ArgoCD Application and Deployment

### Task 16: Create ArgoCD app manifest in nic-test

**Files:**
- Create: `/home/chuck/devel/nic-test/clusters/hetzner-chuck/apps/skillsctl.yaml`

- [ ] **Step 1: Create the ArgoCD Application manifest**

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: skillsctl
  namespace: argocd
  labels:
    app.kubernetes.io/part-of: nebari-foundational
    app.kubernetes.io/managed-by: nebari-infrastructure-core
  annotations:
    argocd.argoproj.io/sync-wave: "6"
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  project: foundational

  source:
    repoURL: https://github.com/nebari-dev/skillsctl.git
    targetRevision: main
    path: chart
    helm:
      releaseName: skillsctl
      values: |
        nebariapp:
          enabled: true
          hostname: skillsctl.hetzner-chuck.openteams.app
          auth:
            enabled: true
            provider: keycloak
            provisionClient: true
            deviceFlowClient:
              enabled: true
            scopes:
              - openid
              - profile
              - email
        server:
          devMode: false
        persistence:
          enabled: true
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 256Mi

  destination:
    server: https://kubernetes.default.svc
    namespace: skillsctl

  syncPolicy:
    managedNamespaceMetadata:
      labels:
        nebari.dev/managed: "true"
    automated:
      prune: true
      selfHeal: true
      allowEmpty: false
    syncOptions:
      - CreateNamespace=true
      - ServerSideApply=true
    retry:
      limit: 5
      backoff:
        duration: 5s
        factor: 2
        maxDuration: 3m
```

- [ ] **Step 2: Verify foundational AppProject allows skillsctl repo**

```bash
cd /home/chuck/devel/nic-test
grep -r "sourceRepos" clusters/ || echo "No sourceRepos restriction found"
```

If restricted, add `https://github.com/nebari-dev/skillsctl.git` to the AppProject's sourceRepos.

- [ ] **Step 3: Commit and push**

```bash
cd /home/chuck/devel/nic-test
git add clusters/hetzner-chuck/apps/skillsctl.yaml
git commit -m "Add skillsctl ArgoCD application for Hetzner cluster"
git push
```

---

### Task 17: Deploy updated operator to Hetzner cluster

**Files:** None

This task depends on the operator PR being merged. The operator's ArgoCD app in the cluster needs to pick up the new version.

- [ ] **Step 1: Add KEYCLOAK_EXTERNAL_URL to operator deployment**

Check how the operator's env vars are configured:

```bash
cd /home/chuck/devel/nic-test
cat clusters/hetzner-chuck/manifests/nebari-operator/*.yaml | grep -A5 "env:"
```

Add `KEYCLOAK_EXTERNAL_URL` to the operator's deployment manifest or Kustomize overlay:

```yaml
- name: KEYCLOAK_EXTERNAL_URL
  value: "https://keycloak.chuck-hetzner.openteams.app/auth"
```

Commit and push to nic-test so ArgoCD picks it up.

- [ ] **Step 2: Check operator pods are running with new version**

```bash
kubectl get pods -n nebari-operator-system
kubectl logs -n nebari-operator-system deploy/nebari-operator-controller-manager --tail=20
```

- [ ] **Step 3: Verify CRD is updated**

```bash
kubectl get crd nebariapps.reconcilers.nebari.dev -o jsonpath='{.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties}' | jq 'keys'
```

Expected: `serviceAccountName` appears in the list

---

### Task 18: Verify end-to-end deployment

**Files:** None

- [ ] **Step 1: Verify ArgoCD synced skillsctl**

```bash
kubectl get application skillsctl -n argocd -o jsonpath='{.status.sync.status}'
```

Expected: `Synced`

- [ ] **Step 2: Verify NebariApp reconciled**

```bash
kubectl get nebariapp -n skillsctl
kubectl describe nebariapp skillsctl -n skillsctl
```

Expected: All conditions True

- [ ] **Step 3: Verify Keycloak clients**

Check Keycloak admin UI or API for:
- Confidential client: `skillsctl-skillsctl`
- Device flow client: `skillsctl-skillsctl-device` (with device authorization grant enabled)

- [ ] **Step 4: Verify Secret contents**

```bash
kubectl get secret skillsctl-oidc-client -n skillsctl -o jsonpath='{.data}' | jq 'keys'
```

Expected: `["client-id", "client-secret", "device-client-id", "issuer-url"]`

- [ ] **Step 5: Verify RBAC**

```bash
kubectl get role,rolebinding -n skillsctl
```

Expected: `skillsctl-oidc-secret-reader` Role and RoleBinding

- [ ] **Step 6: Test CLI auth**

```bash
skillsctl config set registry https://skillsctl.hetzner-chuck.openteams.app
skillsctl auth login
skillsctl auth status
```

Expected: Device flow completes, token is valid

- [ ] **Step 7: Test publish**

```bash
skillsctl publish --name test-skill --version 0.1.0 --description "test" --file /path/to/test.md
```

Expected: Publish succeeds with authenticated user identity
