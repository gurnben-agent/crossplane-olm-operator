# Crossplane OLM Operator

A Kubernetes operator that installs and manages [Crossplane](https://crossplane.io/) on OpenShift through the Operator Lifecycle Manager (OLM). It provides a declarative `CrossplaneConfig` custom resource so cluster admins can manage Crossplane without Helm expertise.

## Features

- **Declarative configuration** — a single `CrossplaneConfig` CR controls the full Crossplane installation
- **Multi-version support** — tracks Crossplane v2.0, v2.1, and v2.2 with per-version Helm value mappings
- **OLM-native lifecycle** — installs via OperatorHub with proper bundle metadata, RBAC, and upgrade graphs
- **Per-Y-stream OLM channels** — `stable-v2.0`, `stable-v2.1`, `stable-v2.2` with automatic Z-stream (patch) upgrades within each channel
- **Automatic upstream sync** — daily CI job detects new Crossplane patch releases and opens a PR
- **Air-gap ready** — Helm charts are embedded in the operator binary via `go:embed`
- **Escape hatch** — `spec.extraHelmValues` provides immediate access to any upstream Helm value before the CR API adds typed fields

## Supported Crossplane Versions

| Version | OLM Channel     | Status    |
|---------|-----------------|-----------|
| v2.0    | `stable-v2.0`   | Supported |
| v2.1    | `stable-v2.1`   | Supported |
| v2.2    | `stable-v2.2`   | Supported (default) |

This operator targets Crossplane 2.x only. Clusters running Crossplane 1.x must upgrade through the upstream path to v2.0 before adopting this operator.

## Installation

### Prerequisites

- OpenShift cluster with OLM installed
- `kubectl` or `oc` CLI

### 1. Create a CatalogSource

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: crossplane-olm-operator
  namespace: openshift-marketplace
spec:
  sourceType: grpc
  image: ghcr.io/gurnben-agent/crossplane-olm-operator-catalog:latest
  displayName: Crossplane OLM Operator
  updateStrategy:
    registryPoll:
      interval: 15m
```

### 2. Subscribe to a channel

Create a Subscription through OperatorHub or apply:

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: crossplane-olm-operator
  namespace: openshift-operators
spec:
  channel: stable-v2.2
  name: crossplane-olm-operator
  source: crossplane-olm-operator
  sourceNamespace: openshift-marketplace
```

### 3. Create a CrossplaneConfig

```yaml
apiVersion: crossplane.io/v1alpha1
kind: CrossplaneConfig
metadata:
  name: crossplane
spec:
  version: "v2.2"
  rbacMode: cluster
  webhooks:
    enabled: true
  features:
    betaDeploymentRuntimeConfigs: true
  packageCache:
    sizeLimit: "5Gi"
```

The operator reconciles this CR into a full Crossplane installation. Only one `CrossplaneConfig` resource may exist (singleton, enforced by webhook).

## Configuration Reference

### CrossplaneConfigSpec

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `version` | `string` | *(required)* | Crossplane version to install (e.g. `"v2.1"`) |
| `providers` | `[]ProviderRef` | `[]` | Providers to install |
| `defaultActivations` | `[]string` | `["*"]` | MRAP activation filter — controls which MR CRDs providers install |
| `packageCache` | `CacheConfig` | — | Cache settings for provider/configuration packages |
| `functionCache` | `CacheConfig` | — | Cache settings for composition function packages |
| `webhooks.enabled` | `*bool` | `true` | Toggle Crossplane webhooks |
| `resources.crossplane` | `ResourceRequirements` | — | Resource budgets for Crossplane core pods |
| `resources.rbacManager` | `ResourceRequirements` | — | Resource budgets for RBAC Manager pods |
| `features` | `FeatureFlags` | — | Crossplane 2.x feature flags (see below) |
| `rbacMode` | `string` | `"cluster"` | RBAC scope: `"namespace"` or `"cluster"` |
| `registry.defaultRegistry` | `string` | `xpkg.crossplane.io` | Default registry for package references |
| `registry.mirror` | `string` | — | Mirror registry URL |
| `registry.pullSecrets` | `[]string` | `[]` | Secret names for image pulls |
| `observability.metricsEnabled` | `*bool` | `true` | Toggle Prometheus metrics |
| `observability.debugEnabled` | `*bool` | `false` | Toggle debug logging |
| `serviceAccount.create` | `*bool` | `true` | Create the Crossplane ServiceAccount |
| `serviceAccount.name` | `string` | — | Override ServiceAccount name |
| `runtimeClassName` | `string` | — | RuntimeClassName for Crossplane pods |
| `extraHelmValues` | `JSON` | — | Raw Helm values merged last (escape hatch) |

### CacheConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `medium` | `string` | `""` (disk) | `""` for disk or `"Memory"` for tmpfs |
| `sizeLimit` | `string` | — | Volume size limit (e.g. `"5Gi"`) |
| `pvc` | `PersistentVolumeClaimSpec` | — | PVC spec for persistent cache |

### FeatureFlags

All flags are optional booleans. If a flag is set for a version that does not support it, the operator ignores it and sets a `FeatureFlagIgnored` status condition.

| Flag | Available from |
|------|----------------|
| `betaUsages` | v2.0 |
| `betaClaimSSA` | v2.0 |
| `betaRealtimeCompositions` | v2.0 |
| `betaDeploymentRuntimeConfigs` | v2.0 |
| `betaCustomToManagedResourceConversion` | v2.0 |
| `alphaOperations` | v2.0 |
| `alphaDependencyVersionUpgrades` | v2.0 |
| `alphaDependencyVersionDowngrades` | v2.0 |
| `alphaSignatureVerification` | v2.0 |
| `alphaFunctionResponseCache` | v2.0 |
| `alphaPipelineInspector` | v2.2 |

## OLM Channels and Upgrades

Each Crossplane minor version has its own OLM channel (`stable-v2.Y`).

- **Z-stream patches** (e.g. v2.1.4 → v2.1.5) are delivered automatically within a channel. The daily sync detects upstream patches, and after the generated PR merges, the catalog image is rebuilt and clusters pick up the update on their next poll cycle.
- **Minor version upgrades** (e.g. v2.1 → v2.2) require switching your Subscription's channel from `stable-v2.1` to `stable-v2.2` and updating `spec.version` in the CR. This is intentional — minor versions may include breaking changes.

## Development

### Prerequisites

- Go 1.26+
- Docker or Podman
- [operator-sdk](https://sdk.operatorframework.io/) v1.39+
- [opm](https://olm.operatorframework.io/docs/cli-tools/opm/) v1.52+
- [KinD](https://kind.sigs.k8s.io/) (for e2e tests)

### Build

```bash
make build          # Build the operator binary
make docker-build   # Build the container image
```

### Test

```bash
make test           # Run codegen checks + unit tests
make test-unit      # Run unit tests only
make lint           # Run golangci-lint
```

### Run locally

```bash
make run            # Run against the configured cluster (uses kubeconfig)
```

### OLM bundle

```bash
make bundle-generate   # Regenerate bundle manifests
make bundle-validate   # Validate all bundles
make catalog-validate  # Validate the FBC catalog
```

### E2E

```bash
make e2e            # Create KinD cluster, install OLM, deploy bundle
make e2e-cleanup    # Delete the KinD cluster
```

### Sync upstream charts

```bash
make sync-upstream  # Pull latest Crossplane Helm charts for each tracked version
```

## CI/CD

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `ci.yml` | Pull request | Lint, vet, unit tests, bundle/catalog validation, e2e |
| `release.yml` | Tag push (`v*`) | Build and push operator, bundle, and catalog images |
| `catalog-publish.yml` | Push to `main` (catalog/bundle changes) | Rebuild and push the FBC catalog image |
| `daily-sync.yml` | Cron (06:00 UTC) | Detect upstream Crossplane patches, open sync PR |

## Architecture

The operator uses the Helm SDK to render vendored Crossplane charts into Kubernetes manifests, then applies them via server-side apply. Each supported Crossplane version has its own value-mapping function that translates the stable CR API into version-specific Helm values.

```
CrossplaneConfig CR
        │
        ▼
  Version Registry  ──►  mapping_v2_X.go  ──►  Helm values
        │                                           │
        ▼                                           ▼
  Embedded Chart  ─────────────────────────►  Helm Render
                                                    │
                                                    ▼
                                          Server-Side Apply
```

For detailed design rationale, see [ADR-0001](docs/adr/0001-olm-operator-for-crossplane.md).
