---
title: "Nebari integration"
weight: 30
---

# Nebari integration

If you are running skillsctl on a cluster managed by [Nebari](https://nebari.dev), you can use the `NebariApp` CRD instead of configuring an Ingress manually. The nebari-operator handles routing, TLS, and OIDC authentication automatically.

## What NebariApp does

When `nebariapp.enabled=true`, the Helm chart creates a `NebariApp` resource. The nebari-operator processes that resource and provisions:

- An `HTTPRoute` to route traffic to the skillsctl service
- A TLS certificate for the hostname
- OIDC authentication via the cluster's Keycloak instance

This means you do not need to configure Ingress, cert-manager, or an external OIDC provider separately. The cluster's existing Keycloak instance becomes the OIDC issuer for skillsctl.

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
helm install skillsctl nebari/skillsctl -f values.yaml
```

The nebari-operator will create the HTTPRoute and TLS certificate. The skillsctl server will be reachable at `https://skills.your-nebari-domain.com`.

## OIDC configuration with NebariApp

When NebariApp is enabled, the nebari-operator configures OIDC using the cluster's Keycloak. Set the OIDC values to match the Keycloak realm:

```yaml
nebariapp:
  enabled: true
  hostname: skills.your-nebari-domain.com

oidc:
  issuerURL: https://keycloak.your-nebari-domain.com/realms/nebari
  clientID: skillsctl
  adminGroup: platform-admins
```

The `clientID` must exist in Keycloak before deploying. Create a client for skillsctl in the Keycloak admin console with:

- Client authentication: on
- Authentication flow: Standard flow + Device authorization grant (for CLI device flow)
- Valid redirect URIs: the skillsctl hostname

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
