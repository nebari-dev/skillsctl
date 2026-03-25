---
title: "Nebari integration"
weight: 30
---

# Nebari integration

If you are running skillsctl on a cluster managed by [Nebari](https://nebari.dev), you can use the `NebariApp` CRD instead of configuring an Ingress manually. The nebari-operator handles routing, TLS, and OIDC authentication automatically.

## Prerequisites

- nebari-operator v0.1.0-alpha.14 or later (deployed by `nic` with `KEYCLOAK_EXTERNAL_URL` configured automatically)
- The target namespace must have the `nebari.dev/managed=true` label (ArgoCD's `managedNamespaceMetadata` handles this automatically)

## What NebariApp does

When `nebariapp.enabled=true`, the Helm chart creates a `NebariApp` resource. The nebari-operator processes that resource and provisions:

- An `HTTPRoute` to route traffic to the skillsctl service
- A TLS certificate for the hostname via cert-manager
- A confidential Keycloak OIDC client for the server
- A public device flow Keycloak client for CLI authentication
- A Kubernetes Secret with client credentials, issuer URL, and device flow client ID
- RBAC (Role + RoleBinding) scoping Secret access to the app's ServiceAccount

This means you do not need to configure Ingress, cert-manager, or Keycloak clients manually.

## When to use NebariApp vs Ingress

Use `NebariApp` when:
- You are deploying to a cluster running the nebari-operator
- You want to reuse the cluster's Keycloak for authentication

Use `ingress.enabled=true` when:
- You are on a non-Nebari cluster
- You have your own Ingress controller and TLS setup
- You are using an external OIDC provider

Do not enable both at the same time.

## Deploying with ArgoCD

Most Nebari clusters use ArgoCD for GitOps. See `examples/argocd-nebari.yaml` in the repository for a complete ArgoCD Application manifest.

The key values:

```yaml
nebariapp:
  enabled: true
  hostname: skillsctl.your-domain.com
  routing:
    routes:
      - pathPrefix: /
  auth:
    enabled: true
    provider: keycloak
    provisionClient: true
    enforceAtGateway: false
    deviceFlowClient:
      enabled: true
    scopes:
      - openid
      - profile
      - email
```

Important settings:
- `routing.routes` is required - without it, no HTTPRoute is created and the service is unreachable
- `enforceAtGateway: false` lets the server validate bearer tokens directly, which is required for CLI device flow authentication
- `deviceFlowClient.enabled: true` provisions a public Keycloak client for `skillsctl auth login`

The ArgoCD app should use `managedNamespaceMetadata` to label the namespace and `ServerSideApply=true` for compatibility with operator-managed resources:

```yaml
syncPolicy:
  managedNamespaceMetadata:
    labels:
      nebari.dev/managed: "true"
  syncOptions:
    - CreateNamespace=true
    - ServerSideApply=true
```

## Deploying with Helm directly

If you prefer Helm over ArgoCD:

```bash
helm install skillsctl oci://ghcr.io/nebari-dev/charts/skillsctl \
  --namespace skillsctl --create-namespace \
  -f values.yaml
```

Or install from the Git repository:

```bash
git clone https://github.com/nebari-dev/skillsctl.git
helm install skillsctl ./skillsctl/chart \
  --namespace skillsctl --create-namespace \
  -f values.yaml
```

Label the namespace for the operator:

```bash
kubectl label namespace skillsctl nebari.dev/managed=true
```

## OIDC configuration

When NebariApp is enabled with `auth.provisionClient: true`, the nebari-operator handles OIDC automatically. You do not need to set `oidc.issuerURL` or `oidc.clientID` in your values - the operator writes them to a Kubernetes Secret, and the Helm chart injects them as environment variables.

The server's `/auth/config` endpoint returns the issuer URL, client ID, and device flow client ID so the CLI can self-configure. Users just run `skillsctl auth login`.

## Verifying the deployment

Check that the NebariApp reconciled successfully:

```bash
kubectl describe nebariapp skillsctl -n skillsctl
```

All conditions should be `True`:
- `Ready` - overall status
- `TLSReady` - certificate provisioned
- `RoutingReady` - HTTPRoute created
- `AuthReady` - OIDC clients provisioned

Check the Secret was created with all expected keys:

```bash
kubectl get secret skillsctl-oidc-client -n skillsctl -o jsonpath='{.data}' | jq 'keys'
```

Expected: `["client-id", "client-secret", "device-client-id", "issuer-url"]`

Check RBAC:

```bash
kubectl get role,rolebinding -n skillsctl
```

Expected: `skillsctl-oidc-secret-reader` Role and RoleBinding.

Test the health endpoint:

```bash
curl https://skillsctl.your-domain.com/healthz
```

Expected: `ok`

Test auth config:

```bash
curl https://skillsctl.your-domain.com/auth/config
```

Expected: JSON with `enabled: true`, `issuer_url`, `client_id`, and `device_client_id`.

## Next steps

- [Configuration reference]({{< relref "/server/configuration" >}}) - environment variables
- [Auth concepts]({{< relref "/concepts/auth" >}}) - how the OIDC device flow works
