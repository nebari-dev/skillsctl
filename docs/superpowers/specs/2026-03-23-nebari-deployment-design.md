# Nebari Deployment: Operator Device Flow, Chart Auth Modes, and ArgoCD App

## Overview

Deploy skillsctl to a Nebari-managed Kubernetes cluster (Hetzner) with full OIDC authentication, while preserving standalone (non-Nebari) deployment support. This requires changes across three repositories:

1. **nebari-operator** - Add device flow client provisioning, Secret RBAC, ServiceAccount field, and issuer URL to Secret
2. **skillsctl** - Update Helm chart to support operator-managed auth alongside standalone OIDC
3. **nic-test** - ArgoCD Application manifest for the Hetzner cluster

## Deployment Modes

The Helm chart supports three mutually exclusive modes:

| Mode | `nebariapp.enabled` | `server.devMode` | OIDC source |
|------|---------------------|-------------------|-------------|
| Standalone + generic OIDC | `false` | `false` | `oidc.*` values in values.yaml, written to ConfigMap |
| Nebari + operator-managed | `true` | `false` | Operator-created K8s Secret |
| Dev (no auth) | N/A | `true` | None |

The server code is identical in all modes - it reads env vars (`OIDC_ISSUER_URL`, `OIDC_CLIENT_ID`, `OIDC_DEVICE_CLIENT_ID`, etc.). The difference is whether those env vars come from the ConfigMap or the Secret.

## Workstream 1: nebari-operator Changes

**GitHub issue:** https://github.com/nebari-dev/nebari-operator/issues/83

### CRD Additions

**New top-level field on NebariAppSpec:**

```go
// ServiceAccountName is the name of the Kubernetes ServiceAccount used by the
// app's pods. Used for RBAC scoping of OIDC secrets. Defaults to the
// NebariApp's name if omitted.
ServiceAccountName string `json:"serviceAccountName,omitempty"`
```

**New field on AuthConfig (alongside existing SPAClient):**

```go
DeviceFlowClient *DeviceFlowClientConfig `json:"deviceFlowClient,omitempty"`

type DeviceFlowClientConfig struct {
    Enabled bool `json:"enabled"`
}
```

### External Issuer URL

The operator's `GetIssuerURL()` returns a cluster-internal URL (e.g. `http://keycloak-keycloakx-http.keycloak.svc.cluster.local:8080/realms/nebari`). This is not usable by CLI clients outside the cluster - it's HTTP (rejected by the CLI's URL validation), and the hostname isn't routable from a developer's laptop.

The operator needs a new cluster-level env var:

```
KEYCLOAK_EXTERNAL_URL=https://keycloak.hetzner-chuck.openteams.app/auth
```

Add a new method `GetExternalIssuerURL()` to the `OIDCProvider` interface. For Keycloak, this constructs `{KEYCLOAK_EXTERNAL_URL}/realms/{realm}`. For generic OIDC, this returns the same value as `GetIssuerURL()` (already external by definition).

The Secret stores the **external** URL. The server uses it for both token validation and the `/auth/config` response. This works because the external URL must be resolvable in-cluster too (the gateway routes it), which is standard for any cluster with ingress.

### Keycloak Provider: Device Flow Client

New method `ProvisionDeviceFlowClient`, mirroring the existing `ProvisionSPAClient` pattern:

- Client ID: `{namespace}-{name}-device`
- `publicClient: true`
- `standardFlowEnabled: false`
- `directAccessGrantsEnabled: false`
- OAuth2 Device Authorization Grant: enabled
- Scopes: derived from `nebariApp.Spec.Auth.Scopes` (openid, profile, email, plus any custom scopes)
- **Audience mapper:** A client-level protocol mapper of type `oidc-audience-mapper` that adds the confidential client's ID (`{namespace}-{name}`) to the `aud` claim. This ensures tokens issued via device flow pass the server's audience validation, which checks against the confidential client ID.

Cleanup: `DeleteClient` must also delete `{namespace}-{name}-device` when a NebariApp is removed, preventing orphan Keycloak clients.

### Secret Contents

The operator-created Secret `{name}-oidc-client` gains new keys:

| Key | Source | When |
|-----|--------|------|
| `client-id` | Confidential client ID | Always (existing) |
| `client-secret` | Confidential client secret | Always (existing) |
| `issuer-url` | `provider.GetExternalIssuerURL()` | Always (new) |
| `device-client-id` | Device flow client ID | When `deviceFlowClient.enabled` (new) |
| `spa-client-id` | SPA client ID | When `spaClient.enabled` (existing) |

### Secret RBAC

After creating the Secret, the auth reconciler creates:

1. **Role** `{name}-oidc-secret-reader`:
   ```yaml
   rules:
     - apiGroups: [""]
       resources: ["secrets"]
       resourceNames: ["{name}-oidc-client"]
       verbs: ["get"]
   ```

2. **RoleBinding** `{name}-oidc-secret-reader`:
   - Binds the Role to ServiceAccount `{spec.serviceAccountName}` (or `{name}` if omitted)

Both resources have ownerReference to the NebariApp for garbage collection. The RoleBinding targets the ServiceAccount in the same namespace as the NebariApp.

The `ServiceAccountName` field should include kubebuilder markers consistent with other optional fields in the CRD:

```go
// +optional
// +kubebuilder:validation:MinLength=1
ServiceAccountName string `json:"serviceAccountName,omitempty"`
```

### Reconciliation Order

No change to the overall order. Within the auth reconciler, RBAC creation slots in after Secret creation:

1. Provision Keycloak clients (confidential, SPA, device flow)
2. Write credentials to Secret
3. Create Role + RoleBinding for Secret access

## Workstream 2: skillsctl Helm Chart Changes

### values.yaml Additions

```yaml
oidc:
  issuerURL: ""
  clientID: ""
  deviceClientID: ""    # NEW: for standalone device flow client
  adminGroup: "skillsctl-admins"
  groupsClaim: "groups"

nebariapp:
  enabled: false
  # hostname: skillsctl.nebari.example.com
  serviceAccountName: ""  # NEW: passed to NebariApp CRD, defaults to release fullname
  auth:
    enabled: true
    provider: keycloak
    provisionClient: true
    deviceFlowClient:       # NEW
      enabled: true
    redirectURI: /
    scopes:
      - openid
      - profile
      - email
  # ... rest unchanged
```

### ConfigMap (configmap.yaml)

Only includes OIDC client values when nebariapp is disabled (standalone mode). Admin/groups config is always included:

```yaml
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

### Deployment (deployment.yaml)

When nebariapp is enabled, inject OIDC env vars from the operator-created Secret. The Secret is created by the operator after reconciling the NebariApp, so the pod may initially fail to start until the Secret exists - this is expected and Kubernetes will retry.

Note: The existing `checksum/config` annotation only triggers rollouts on ConfigMap changes. Since the Secret is operator-managed (not rendered by Helm), changes to it won't automatically restart pods. This is acceptable because OIDC credentials rarely change, and the operator can be extended with a restart annotation in the future if needed.

```yaml
containers:
  - name: {{ .Chart.Name }}
    envFrom:
      - configMapRef:
          name: {{ include "skillsctl.fullname" . }}
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

### NebariApp Template (nebariapp.yaml)

Add serviceAccountName and deviceFlowClient fields:

```yaml
spec:
  hostname: {{ .Values.nebariapp.hostname }}
  serviceAccountName: {{ .Values.nebariapp.serviceAccountName | default (include "skillsctl.fullname" .) }}
  service:
    name: {{ .Values.nebariapp.service.name | default (include "skillsctl.fullname" .) }}
    port: {{ .Values.nebariapp.service.port | default 80 }}
  auth:
    enabled: {{ .Values.nebariapp.auth.enabled }}
    provider: {{ .Values.nebariapp.auth.provider }}
    provisionClient: {{ .Values.nebariapp.auth.provisionClient }}
    {{- if .Values.nebariapp.auth.deviceFlowClient }}
    deviceFlowClient:
      enabled: {{ .Values.nebariapp.auth.deviceFlowClient.enabled }}
    {{- end }}
    # ... rest unchanged
```

### Backend Code Changes

**`main.go`:** Read new env var `OIDC_DEVICE_CLIENT_ID` (defaulting to empty string).

**`backend/internal/auth/config.go`:** Add `DeviceClientID string` field to `Config` struct.

**`backend/internal/server/server.go` (or equivalent auth config handler):** Add `device_client_id` to the `/auth/config` JSON response:

```json
{
  "issuer_url": "https://keycloak.example.com/realms/nebari",
  "client_id": "skillsctl-skillsctl",
  "device_client_id": "skillsctl-skillsctl-device"
}
```

If `OIDC_DEVICE_CLIENT_ID` is empty, `device_client_id` is omitted from the response (backwards compatible). Token validation continues to use `client_id` (the confidential client) as the expected audience - device flow tokens pass this check because the operator configures an audience mapper on the device flow client (see Workstream 1).

## Workstream 3: ArgoCD Application

### File: `clusters/hetzner-chuck/apps/skillsctl.yaml` in nic-test repo

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

**Namespace creation:** ArgoCD creates the `skillsctl` namespace automatically via `CreateNamespace=true` and applies the `nebari.dev/managed=true` label via `managedNamespaceMetadata`, matching the established pattern (e.g. `pi-agent.yaml`).

**ServerSideApply:** Included for consistency with other apps in the cluster (keycloak, nebari-operator) and to avoid field ownership conflicts with operator-managed resources.

**Sync wave 6:** Keycloak is wave 4, nebari-operator is wave 5. Skillsctl must sync after both because the operator needs to be running to reconcile the NebariApp CRD, and Keycloak must be healthy to accept client provisioning requests.

**Repo access:** The `foundational` ArgoCD AppProject may need `https://github.com/nebari-dev/skillsctl.git` added to its `sourceRepos` list, since this is the first app sourced from outside the nic-test repo. Verify before deploying.

## Implementation Order

1. **nebari-operator** - CRD changes, device flow client provisioning, issuer URL in Secret, RBAC (issue #83)
2. **skillsctl chart** - ConfigMap conditional, Deployment Secret refs, NebariApp template updates, /auth/config device_client_id
3. **nic-test** - ArgoCD Application manifest

Workstreams 1 and 2 can be developed in parallel since they touch different repos. Workstream 3 depends on both being merged.

## Testing

- **Operator:** Unit tests for device flow client provisioning, audience mapper configuration, RBAC creation, ServiceAccount defaulting, device flow client cleanup on delete. Integration test with Keycloak test container.
- **Chart:** Add CI test values files for all three modes:
  - `chart/ci/test-values.yaml` - existing standalone mode (update to include `oidc.deviceClientID`)
  - `chart/ci/nebari-values.yaml` - nebariapp enabled mode
  - `chart/ci/devmode-values.yaml` - dev mode
  - Run `helm template` for each to verify ConfigMap/Deployment output is correct per mode.
- **Backend:** Unit tests for `/auth/config` returning `device_client_id` when set, omitting when empty.
- **End-to-end:** Deploy to Hetzner cluster, verify `skillsctl auth login` completes device flow, `skillsctl publish` succeeds with valid token, `/auth/config` returns all three fields.
