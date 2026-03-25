---
title: "Nebari integration"
weight: 30
---

# Nebari integration

If you are running SkillsCtl on a cluster managed by [Nebari](https://nebari.dev), you can use the `NebariApp` CRD instead of configuring an Ingress manually. The nebari-operator handles routing, TLS, and OIDC authentication automatically.

## What NebariApp does

When `nebariapp.enabled=true`, the Helm chart creates a `NebariApp` resource. The nebari-operator processes that resource and provisions:

- An `HTTPRoute` to route traffic to the SkillsCtl service
- A TLS certificate for the hostname
- OIDC authentication via the cluster's Keycloak instance

This means you do not need to configure Ingress, cert-manager, or an external OIDC provider separately. The cluster's existing Keycloak instance becomes the OIDC issuer for SkillsCtl.

## When to use NebariApp vs Ingress

Use `NebariApp` when:
- You are deploying to a cluster running the nebari-operator
- You want to reuse the cluster's Keycloak for authentication

Use `ingress.enabled=true` when:
- You are on a non-Nebari cluster
- You have your own Ingress controller and TLS setup
- You are using an external OIDC provider

Do not enable both at the same time. NebariApp creates its own HTTPRoute, and combining it with an Ingress will result in conflicting routing rules.

## Enabling NebariApp

Set the required values in your `values.yaml`:

```yaml
nebariapp:
  enabled: true
  hostname: skills.your-nebari-domain.com
```

Keep `ingress.enabled` at its default (`false`).

Deploy:

```bash
helm install SkillsCtl nebari/skillsctl -f values.yaml
```

The nebari-operator will create the HTTPRoute and TLS certificate. The SkillsCtl server will be reachable at `https://skills.your-nebari-domain.com`.

## OIDC configuration with NebariApp

When NebariApp is enabled with `auth.provisionClient: true`, the nebari-operator handles OIDC automatically:

- Provisions a confidential Keycloak client for the server
- Provisions a public device flow client for CLI authentication (when `deviceFlowClient.enabled: true`)
- Writes client credentials and issuer URL to a Kubernetes Secret
- The Helm chart injects these from the Secret as environment variables

You do not need to set `oidc.issuerURL` or `oidc.clientID` manually. The operator manages them.

Example values for a Nebari deployment:

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

Setting `enforceAtGateway: false` means the server validates bearer tokens directly, which is required for CLI device flow authentication.

See `examples/argocd-nebari.yaml` in the repository for a complete ArgoCD Application example.

## Verifying the deployment

Once the operator has processed the NebariApp resource, check the HTTPRoute:

```bash
kubectl get httproute -l app.kubernetes.io/name=skillsctl
```

Then verify the health endpoint:

```bash
curl https://skills.your-nebari-domain.com/healthz
```

Expected response: `ok`

## Next steps

- [Configuration reference]({{< relref "/server/configuration" >}}) - OIDC environment variables
- [Auth concepts]({{< relref "/concepts/auth" >}}) - how the OIDC device flow works
