---
title: "Kubernetes (Helm)"
weight: 20
---

# Kubernetes (Helm)

SkillsCtl ships a Helm chart for deploying the registry server to Kubernetes.

## Prerequisites

- Kubernetes 1.24 or later
- Helm 3.10 or later
- A storage class that supports `ReadWriteOnce` (for single-replica) or `ReadWriteMany` (for HA)

## Install

Add the Helm repository and install:

```bash
helm repo add nebari https://nebari-dev.github.io/helm-repository
helm repo update
helm install SkillsCtl nebari/skillsctl
```

Alternatively, install from the OCI registry:

```bash
helm install SkillsCtl oci://quay.io/nebari/skillsctl
```

The default install creates:

- 1 replica
- A 1Gi `ReadWriteOnce` PVC named `skillsctl-data`
- No Ingress (you configure external access separately)
- No OIDC (dev mode - auth disabled)

## Verify the deployment

```bash
kubectl get pods -l app.kubernetes.io/name=skillsctl
kubectl exec -it deploy/skillsctl -- wget -qO- localhost:8080/healthz
```

## Configuration values

Pass values with `--set` or a `values.yaml` file (`helm install SkillsCtl nebari/skillsctl -f values.yaml`).

### Image

| Value | Default | Description |
|-------|---------|-------------|
| `image.repository` | `ghcr.io/nebari-dev/skillsctl-backend` | Container image repository |
| `image.tag` | chart app version | Image tag |
| `image.pullPolicy` | `IfNotPresent` | Image pull policy |

### Replicas

| Value | Default | Description |
|-------|---------|-------------|
| `replicaCount` | `1` | Number of server replicas |

For more than 1 replica, set a `ReadWriteMany` storage class. See [High availability](#high-availability) below.

### Persistence

| Value | Default | Description |
|-------|---------|-------------|
| `persistence.enabled` | `true` | Create a PVC for the SQLite database |
| `persistence.storageClassName` | `""` (cluster default) | Storage class for the PVC |
| `persistence.accessMode` | `ReadWriteOnce` | PVC access mode |
| `persistence.size` | `1Gi` | PVC size |

The PVC is annotated with `helm.sh/resource-policy: keep`, so it survives `helm uninstall`. This protects your skill data from accidental deletion. To delete the PVC, remove it manually:

```bash
kubectl delete pvc skillsctl-data
```

### OIDC

| Value | Default | Description |
|-------|---------|-------------|
| `oidc.issuerURL` | `""` | OIDC issuer URL. Leave empty for dev mode. |
| `oidc.clientID` | `""` | OIDC client ID |
| `oidc.adminGroup` | `""` | Group name that grants admin access |
| `oidc.groupsClaim` | `groups` | JWT claim name containing group membership |

Set `oidc.issuerURL` and `oidc.clientID` to enable authentication. See [Configuration reference]({{< relref "/server/configuration" >}}) for details on OIDC setup.

### Ingress

| Value | Default | Description |
|-------|---------|-------------|
| `ingress.enabled` | `false` | Create an Ingress resource |
| `ingress.className` | `""` | Ingress class name |
| `ingress.hostname` | `""` | Hostname for the Ingress rule |
| `ingress.tls` | `[]` | TLS configuration |

If you are running on a Nebari cluster, use the [NebariApp integration]({{< relref "/server/nebari" >}}) instead of enabling Ingress.

### NebariApp

| Value | Default | Description |
|-------|---------|-------------|
| `nebariapp.enabled` | `false` | Create a NebariApp resource |
| `nebariapp.hostname` | `""` | Hostname for the NebariApp |

See [Nebari integration]({{< relref "/server/nebari" >}}) for details.

## Example: production deployment with OIDC

```yaml
# values-prod.yaml
replicaCount: 1

persistence:
  size: 5Gi

oidc:
  issuerURL: https://keycloak.example.com/realms/myrealm
  clientID: SkillsCtl
  adminGroup: platform-admins

ingress:
  enabled: true
  className: nginx
  hostname: skills.example.com
  tls:
    - secretName: skillsctl-tls
      hosts:
        - skills.example.com
```

```bash
helm install SkillsCtl nebari/skillsctl -f values-prod.yaml
```

## High availability

SQLite handles concurrent writes through WAL (write-ahead logging) mode and a 5-second busy timeout. Multiple replicas can share a database file when the storage class supports `ReadWriteMany`.

To run multiple replicas:

```yaml
# values-ha.yaml
replicaCount: 3

persistence:
  accessMode: ReadWriteMany
  storageClassName: efs  # or nfs, azurefile, etc.
  size: 10Gi
```

Write throughput is bounded by SQLite's single-writer model. For a skill registry, this is rarely a bottleneck: reads dominate, and writes (publishing new versions) are infrequent. A 1Gi database stores roughly 50,000 skills at 10KB average content size.

## Upgrading

```bash
helm repo update
helm upgrade SkillsCtl nebari/skillsctl -f values.yaml
```

## Uninstalling

```bash
helm uninstall SkillsCtl
```

The PVC is retained due to `helm.sh/resource-policy: keep`. Delete it manually if you want to remove all data.

## Next steps

- [Nebari integration]({{< relref "/server/nebari" >}}) - NebariApp CRD for Nebari clusters
- [Configuration reference]({{< relref "/server/configuration" >}}) - all environment variables and OIDC setup
