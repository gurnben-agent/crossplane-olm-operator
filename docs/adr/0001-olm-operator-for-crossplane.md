# 0001 — OLM Operator for Crossplane on OpenShift

## Status

Accepted

## Context

Crossplane has no official OLM packaging for OpenShift. Teams wanting Crossplane
on OpenShift must manually install via Helm, losing lifecycle management,
upgrade-path governance, and the admin-friendly configuration surface that
OpenShift operators typically provide (see RHACM's MultiClusterHub CR as the
reference UX).

We need a single operator that:

- Installs and manages Crossplane through a declarative CR
- Exposes common tuning knobs without requiring Helm expertise
- Publishes proper OLM bundles and an FBC catalog so clusters can consume
  it through the standard OperatorHub flow
- Tracks the 3 most recent Crossplane minor releases (currently v2.0–v2.2)
- Rebuilds daily to pick up upstream patch releases automatically

Crossplane 2.x is a major architectural shift from 1.x: namespaced resources by
default, Managed Resource Activation Policies (MRAP) for CRD filtering, a new
`ops.crossplane.io` API group for day-two operations, removal of
`ControllerConfig` and `StoreConfig` CRDs, and a new image registry
(`xpkg.crossplane.io`). The operator must be designed for the 2.x world from
the start — there is no 1.x compatibility target.

## Decision

### 1. Repository layout — single repo, `crossplane-olm-operator`

```
crossplane-olm-operator/
├── api/
│   └── v1alpha1/
│       ├── crossplaneconfig_types.go
│       ├── groupversion_info.go
│       └── zz_generated.deepcopy.go
├── internal/
│   ├── controller/
│   │   └── crossplaneconfig_controller.go
│   ├── helm/
│   │   ├── renderer.go          # Helm SDK chart render + diff
│   │   └── applier.go           # Server-side-apply of rendered manifests
│   └── version/
│       ├── registry.go          # Version registry: maps CR .spec.version → chart + value map
│       └── mapping_v2_0.go      # Per-version value-mapping functions
│       └── mapping_v2_1.go
│       └── mapping_v2_2.go
├── charts/                      # Vendored upstream Helm charts (one subdir per version)
│   ├── v2.0/
│   ├── v2.1/
│   └── v2.2/
├── bundle/                      # OLM bundle manifests
│   ├── v2.0/
│   │   ├── manifests/
│   │   │   ├── crossplane-olm-operator.clusterserviceversion.yaml
│   │   │   └── crossplaneconfigs.crossplane.io.crd.yaml
│   │   ├── metadata/
│   │   │   └── annotations.yaml
│   │   └── bundle.Dockerfile
│   ├── v2.1/
│   └── v2.2/
├── catalog/
│   └── crossplane-olm-operator/
│       └── catalog.yaml         # FBC fragment (olm.package + olm.channel + olm.bundle entries)
├── hack/
│   ├── sync-upstream.sh         # Pulls latest Crossplane Helm chart for each version
│   └── generate-bundle.sh       # Regenerates bundle manifests from operator-sdk
├── .github/workflows/
│   ├── ci.yml                   # PR gate: lint, unit, e2e
│   ├── release.yml              # Tag-triggered: build operator image, bundles, catalog
│   ├── catalog-publish.yml      # Main-push: rebuild catalog image for Z-stream delivery
│   └── daily-sync.yml           # Cron: detect upstream changes, open PR
├── Dockerfile                   # Operator image
├── Makefile
└── go.mod
```

Everything lives in one repository because the operator image, bundle images,
and catalog fragment share the same release cadence and versioning. Splitting
repos would introduce cross-repo coordination overhead with no isolation
benefit.

### 2. Operator design

**Scaffolding**: `operator-sdk init` with Go plugin, single-group API
`crossplane.io/v1alpha1`, kind `CrossplaneConfig`.

**CRD — `CrossplaneConfig`** (cluster-scoped, singleton pattern enforced by
webhook):

```go
type CrossplaneConfigSpec struct {
    // Version of Crossplane to install (e.g. "v2.1"). Required.
    Version string `json:"version"`

    // Provider management
    Providers []ProviderRef `json:"providers,omitempty"`

    // Managed Resource Activation Policy — controls which MR CRDs providers
    // install. Default ["*"] installs all. Set to specific MR types to reduce
    // CRD footprint (new in Crossplane 2.0). Maps to the Helm chart's
    // provider.defaultActivations value, which creates a default MRAP resource.
    DefaultActivations []string `json:"defaultActivations,omitempty"`

    // Package cache (for provider/configuration packages)
    PackageCache CacheConfig `json:"packageCache,omitempty"`

    // Function cache (for composition function packages, new in 2.x)
    FunctionCache CacheConfig `json:"functionCache,omitempty"`

    // Webhooks
    Webhooks WebhookConfig `json:"webhooks,omitempty"`

    // Resource budgets for Crossplane pods
    Resources ResourceConfig `json:"resources,omitempty"`

    // Feature flags (2.x flags — see FeatureFlags struct)
    Features FeatureFlags `json:"features,omitempty"`

    // RBAC mode: "namespace" or "cluster"
    RBACMode string `json:"rbacMode,omitempty"`

    // Registry overrides (mirrors, pull secrets)
    Registry RegistryConfig `json:"registry,omitempty"`

    // Metrics and debug
    Observability ObservabilityConfig `json:"observability,omitempty"`

    // ServiceAccount configuration
    ServiceAccount ServiceAccountConfig `json:"serviceAccount,omitempty"`

    // RuntimeClassName for Crossplane and RBAC Manager pods
    RuntimeClassName string `json:"runtimeClassName,omitempty"`

    // Escape hatch: raw Helm values merged last, overriding any mapped values.
    // Provides immediate access to new upstream Helm values before the CR API
    // adds typed fields. Unvalidated — use at your own risk.
    ExtraHelmValues *apiextensionsv1.JSON `json:"extraHelmValues,omitempty"`
}
```

Key sub-structs:

```go
// FeatureFlags maps to Crossplane 2.x --enable-* args.
// Flags that were GA'd and removed (e.g. EnvironmentConfigs, CompositionRevisions,
// CompositionWebhookSchemaValidation) are intentionally absent — they are always-on.
type FeatureFlags struct {
    BetaUsages                            *bool `json:"betaUsages,omitempty"`
    BetaClaimSSA                          *bool `json:"betaClaimSSA,omitempty"`
    BetaRealtimeCompositions              *bool `json:"betaRealtimeCompositions,omitempty"`
    BetaDeploymentRuntimeConfigs          *bool `json:"betaDeploymentRuntimeConfigs,omitempty"`
    BetaCustomToManagedResourceConversion *bool `json:"betaCustomToManagedResourceConversion,omitempty"`
    AlphaOperations                       *bool `json:"alphaOperations,omitempty"`
    AlphaDependencyVersionUpgrades        *bool `json:"alphaDependencyVersionUpgrades,omitempty"`
    AlphaDependencyVersionDowngrades      *bool `json:"alphaDependencyVersionDowngrades,omitempty"`
    AlphaSignatureVerification            *bool `json:"alphaSignatureVerification,omitempty"`
    AlphaFunctionResponseCache            *bool `json:"alphaFunctionResponseCache,omitempty"`
    AlphaPipelineInspector                *bool `json:"alphaPipelineInspector,omitempty"` // v2.2+
}

type CacheConfig struct {
    Medium    string                            `json:"medium,omitempty"` // "" (disk) or "Memory"
    SizeLimit string                            `json:"sizeLimit,omitempty"`
    PVC       *corev1.PersistentVolumeClaimSpec `json:"pvc,omitempty"`
}

type RegistryConfig struct {
    // Default registry for package references (default: xpkg.crossplane.io)
    DefaultRegistry string   `json:"defaultRegistry,omitempty"`
    Mirror          string   `json:"mirror,omitempty"`
    PullSecrets     []string `json:"pullSecrets,omitempty"`
}

type ServiceAccountConfig struct {
    Create bool   `json:"create,omitempty"` // default true
    Name   string `json:"name,omitempty"`   // override name when create=false
}
```

**Reconciliation loop** (`crossplaneconfig_controller.go`):

1. Fetch the singleton `CrossplaneConfig` CR.
2. Look up the version registry to get the matching embedded Helm chart path
   and value-mapping function.
3. Call the mapping function to translate CR spec fields → Helm `values.yaml`
   overrides. Each version has its own mapping function because Crossplane's
   Helm values schema changes between minors.
4. Render the chart with `helm/v3` SDK (`action.Install` / `action.Upgrade` in
   template-only mode) to produce a slice of `unstructured.Unstructured`.
5. Diff against the live cluster state (3-way merge via server-side apply field
   managers).
6. Apply the diff. Use server-side apply with a unique field manager
   (`crossplane-olm-operator`) so that user-applied overrides on non-managed
   fields are preserved.
7. Report status on the CR: `Ready`, `Progressing`, `Degraded`,
   `FeatureFlagIgnored` conditions plus `observedVersion`,
   `observedGeneration`, and `helmReleaseDigest` (SHA-256 of the rendered
   manifest set — enables quick external verification that live state matches
   the embedded chart).

**Leader election**: Enabled via operator-sdk scaffolding defaults
(`leader-election-id: crossplane-olm-operator`). Liveness probe at `/healthz`,
readiness probe at `/readyz`, both wired by the default operator-sdk health
check setup.

**Operator RBAC requirements**: The operator ServiceAccount requires the
following permissions (defined in the CSV's ClusterPermissions):

| API Group | Resources | Verbs |
|---|---|---|
| `crossplane.io` | `crossplaneconfigs`, `crossplaneconfigs/status`, `crossplaneconfigs/finalizers` | get, list, watch, update, patch |
| `apps` | `deployments` | get, list, watch, create, update, patch, delete |
| `""` (core) | `serviceaccounts`, `services`, `configmaps`, `secrets`, `persistentvolumeclaims` | get, list, watch, create, update, patch, delete |
| `""` (core) | `namespaces` | get, list, watch, create |
| `""` (core) | `events` | create, patch |
| `rbac.authorization.k8s.io` | `clusterroles`, `clusterrolebindings`, `roles`, `rolebindings` | get, list, watch, create, update, patch, delete |
| `apiextensions.k8s.io` | `customresourcedefinitions` | get, list, watch, create, update, patch, delete |
| `admissionregistration.k8s.io` | `validatingwebhookconfigurations`, `mutatingwebhookconfigurations` | get, list, watch, create, update, patch, delete |
| `coordination.k8s.io` | `leases` | get, list, watch, create, update, patch, delete |

The operator manages Crossplane's 21 CRDs (as of 2.x) spanning these API
groups: `apiextensions.crossplane.io` (XRDs, Compositions, CompositionRevisions,
EnvironmentConfigs, MRAPs, MRDs), `pkg.crossplane.io` (Providers, Functions,
Configurations, their revisions, DeploymentRuntimeConfigs, ImageConfigs, Locks),
`ops.crossplane.io` (Operations, CronOperations, WatchOperations — new in 2.0),
and `protection.crossplane.io` (Usages, ClusterUsages — new in 2.0). Note:
Usages moved from `apiextensions.crossplane.io` to `protection.crossplane.io`
in 2.0; both the old and new CRDs coexist in 2.x for backward compatibility,
but the operator only references the `protection.crossplane.io` versions. The
removed 1.x CRDs (`ControllerConfig`, `StoreConfig`) are not referenced.

**Singleton enforcement**: A validating webhook rejects creation of a second
`CrossplaneConfig` resource. The webhook uses `failurePolicy: Ignore` so that
operator downtime does not block CR operations — acceptable because the
singleton invariant is also enforced by the controller (which only reconciles
the oldest instance). The webhook improves UX with a clear admission error
during normal operation but degrades gracefully when unavailable.

**Finalizer**: The controller adds a finalizer to handle teardown — it
uninstalls the rendered Helm manifests in reverse dependency order before
removing the finalizer.

**Helm chart embedding**: Charts are vendored into `charts/<version>/` at build
time by `hack/sync-upstream.sh` and embedded via `go:embed`. This ensures
air-gap support — no runtime chart downloads.

### 3. Version-specific value mappings

Each `mapping_v2_X.go` file implements:

```go
func MapV2_1(spec *v1alpha1.CrossplaneConfigSpec) (map[string]interface{}, []IgnoredField, error)
```

This function knows the exact Helm `values.yaml` schema for that Crossplane
release and translates the stable CR API into it. When upstream renames or
restructures a value key, only the mapping file for that version changes — the
CR API stays stable for users.

**Known mapping differences across v2.0–v2.2** (exhaustive as of this writing —
Agent B must verify by diffing the vendored `values.yaml` files and update this
table if additional differences are found):

| Mapping concern | v2.0 | v2.1 | v2.2 |
|---|---|---|---|
| `AlphaPipelineInspector` flag | ignored (not available) | ignored | mapped to `--enable-alpha-pipeline-inspector` |
| `sidecarsCrossplane` value | not available | not available | mapped from observability config |
| `secrets.customAnnotations` | not available | not available | mapped if registry annotations set |
| `--max-reconcile-rate` naming | `--max-reconcile-rate` | `--max-concurrent-reconciles` | `--max-concurrent-reconciles` |

**Feature flag handling across versions**: If a user sets a flag that does not
exist in their target version (e.g. `AlphaPipelineInspector` on v2.0), the
mapping function omits it from the Helm values and returns it in the
`[]IgnoredField` list. The controller sets a `FeatureFlagIgnored` status
condition with a message listing each ignored flag and the reason (e.g.
"alphaPipelineInspector: flag not available until v2.2"). This is a warning, not
an error — reconciliation proceeds. The validating webhook does not reject
unknown flags because the same CR must remain valid across version changes.

If `spec.extraHelmValues` is set, its contents are deep-merged into the mapped
values after the mapping function runs. This allows immediate access to new
upstream values. Conflicts are resolved in favor of `extraHelmValues`.

The version registry (`registry.go`) maps `spec.version` strings to the correct
`(chartFS, mapFunc)` pair and returns an error for unsupported versions.

### 4. OLM bundle

One bundle directory per supported Crossplane minor version. Each bundle
contains:

- **CSV** (`clusterserviceversion.yaml`): operator deployment spec, RBAC
  (ClusterRole for CRD management, namespace-scoped Roles for Crossplane
  workloads), owned CRDs (`CrossplaneConfig`), install modes
  (OwnNamespace + AllNamespaces), icon, description, maturity, links.
- **CRD snapshot**: The `CrossplaneConfig` CRD YAML, generated by
  `controller-gen` and identical across bundles (the CRD is version-agnostic;
  the *operator* knows which Crossplane versions it supports).
- **annotations.yaml**: `operators.operatorframework.io.bundle.mediatype.v1`,
  `operators.operatorframework.io.bundle.manifests.v1`,
  `operators.operatorframework.io.bundle.metadata.v1`, package name, channel
  (`stable-v2.Y` matching the bundle's target Crossplane minor).

Bundle images are built with `bundle.Dockerfile` and pushed to
`ghcr.io/<org>/crossplane-olm-operator-bundle:<version>`.

### 5. FBC catalog fragment

`catalog/crossplane-olm-operator/catalog.yaml` uses the FBC schema:

```yaml
schema: olm.package
name: crossplane-olm-operator
defaultChannel: stable-v2.2

---
schema: olm.channel
name: stable-v2.0
package: crossplane-olm-operator
entries:
  - name: crossplane-olm-operator.v2.0.0
  - name: crossplane-olm-operator.v2.0.1
    replaces: crossplane-olm-operator.v2.0.0
  # ... Z-stream entries appended by hack/generate-bundle.sh as upstream patches land

---
schema: olm.channel
name: stable-v2.1
package: crossplane-olm-operator
entries:
  - name: crossplane-olm-operator.v2.1.0
  - name: crossplane-olm-operator.v2.1.1
    replaces: crossplane-olm-operator.v2.1.0
  # ... Z-stream entries appended automatically

---
schema: olm.channel
name: stable-v2.2
package: crossplane-olm-operator
entries:
  - name: crossplane-olm-operator.v2.2.0
  - name: crossplane-olm-operator.v2.2.1
    replaces: crossplane-olm-operator.v2.2.0
  # ... Z-stream entries appended automatically

---
schema: olm.bundle
name: crossplane-olm-operator.v2.0.0
package: crossplane-olm-operator
image: ghcr.io/<org>/crossplane-olm-operator-bundle:v2.0.0

---
schema: olm.bundle
name: crossplane-olm-operator.v2.1.0
package: crossplane-olm-operator
image: ghcr.io/<org>/crossplane-olm-operator-bundle:v2.1.0

---
schema: olm.bundle
name: crossplane-olm-operator.v2.2.0
package: crossplane-olm-operator
image: ghcr.io/<org>/crossplane-olm-operator-bundle:v2.2.0
```

**Channel strategy — per-Y-stream with automatic Z-stream updates**:

Each Crossplane minor version (Y-stream) has its own OLM channel:
`stable-v2.0`, `stable-v2.1`, `stable-v2.2`. The `defaultChannel` is the
latest (`stable-v2.2`). Users subscribe to the channel matching their desired
Crossplane minor version. Within a channel, Z-stream (patch) releases form a
linear `replaces` chain — OLM automatically upgrades subscribers to the latest
patch when the catalog image is updated.

- **Cross-minor upgrades** require the user to switch their Subscription's
  channel (e.g. `stable-v2.1` → `stable-v2.2`). This is intentional: minor
  version bumps in Crossplane can include breaking changes (new CRDs, removed
  features, behavioral changes) and should not happen without user intent.
- **Z-stream patches** are automatic within a channel. When the daily sync
  detects a new upstream patch (e.g. Crossplane v2.1.4 → v2.1.5), the sync PR
  adds a new bundle entry to the `stable-v2.1` channel with
  `replaces: crossplane-olm-operator.v2.1.4`. After the PR merges and the
  catalog image is rebuilt, clusters with automatic approval pick up the
  patch on their next catalog poll.
- **No cross-channel replaces**: bundle entries do not cross channel boundaries.
  `stable-v2.1` entries never reference `stable-v2.0` bundles. Each channel is
  a self-contained upgrade graph.

**Periodic Z-stream publishing flow**:

1. **Daily sync** (06:00 UTC): `daily-sync.yml` runs `hack/sync-upstream.sh`,
   which checks each tracked minor for new upstream patches (see §6).
2. **PR creation**: if new patches are found, a PR is opened with updated
   vendored charts, regenerated bundles (version bumped), and an updated FBC
   catalog fragment with new entries appended to the affected channel(s).
3. **CI validation**: the PR triggers `ci.yml` — lint, bundle validate, FBC
   validate, e2e.
4. **Merge**: after CI passes, the PR is eligible for merge (manually or via
   auto-merge if configured).
5. **Catalog rebuild**: merging to main triggers the `catalog-publish.yml`
   workflow, which rebuilds and pushes the catalog image to
   `ghcr.io/<org>/crossplane-olm-operator-catalog:latest`. Clusters polling
   this image pick up the new Z-stream bundles on their next poll cycle
   (default 15 minutes).

The catalog fragment is built into an image with `opm` and pushed to
`ghcr.io/<org>/crossplane-olm-operator-catalog:latest` (plus semver tags).
Clusters add a `CatalogSource` pointing at this image.

### 6. CI/CD — GitHub Actions

**`ci.yml`** (on PR):
- Go lint (`golangci-lint`), vet, unit tests
- `controller-gen` CRD generation check (ensure generated files are committed)
- Bundle validation (`operator-sdk bundle validate`)
- FBC validation (`opm validate catalog/`)
- E2E: spin up a KinD cluster with OLM installed, deploy the bundle, create a
  `CrossplaneConfig` CR, assert Crossplane pods reach Ready

**`release.yml`** (on tag push `v*`):
- Build and push multi-arch operator image →
  `ghcr.io/<org>/crossplane-olm-operator:<tag>`
- Build and push bundle images (one per supported Crossplane version) →
  `ghcr.io/<org>/crossplane-olm-operator-bundle:<cp-version>`
- Build and push FBC catalog image →
  `ghcr.io/<org>/crossplane-olm-operator-catalog:<tag>`

**`catalog-publish.yml`** (on push to `main` when `catalog/` or `bundle/`
files change):
- Rebuild and push the FBC catalog image to
  `ghcr.io/<org>/crossplane-olm-operator-catalog:latest`
- This is the trigger that makes Z-stream patches available to clusters —
  once the daily-sync PR merges to main, this workflow publishes the updated
  catalog within minutes. Clusters polling `:latest` pick up the new bundles
  on their next `CatalogSource` poll cycle.

**`daily-sync.yml`** (cron, 06:00 UTC):
- Run `hack/sync-upstream.sh` which for each tracked minor (v2.0, v2.1,
  v2.2):
  1. Fetches the upstream Crossplane Helm chart index from
     `https://charts.crossplane.io/stable`
  2. Compares the `appVersion` field in the upstream `Chart.yaml` against the
     vendored `charts/<version>/Chart.yaml`
  3. If the upstream `appVersion` is newer: pulls the new chart tarball,
     extracts it into `charts/<version>/`, regenerates the bundle (bumping
     the bundle version to match the new patch), appends a new entry with
     `replaces` to the affected channel in the FBC catalog fragment, and
     builds + validates the new bundle image
- If any version changed: commits all updates and opens a PR titled
  `chore: sync upstream crossplane charts (YYYY-MM-DD)`
- The PR triggers `ci.yml`, so the full validation suite runs before merge
- After merge, `catalog-publish.yml` pushes the updated catalog image,
  completing the Z-stream publishing pipeline

### 7. Error handling strategy

| Scenario | Handling |
|---|---|
| Unsupported `spec.version` | Reject at webhook admission; controller sets `Degraded` condition with message |
| Helm render failure | Requeue with backoff; set `Degraded` condition; emit Event |
| Server-side apply conflict | Requeue; log field-manager conflict details; do not force-override user fields |
| Upstream chart missing in embed | Build-time failure (CI catches this before release) |
| Finalizer teardown partial failure | Requeue deletion; leave finalizer until all child resources are confirmed removed |

### 8. Crossplane 2.x-specific design considerations

**Image registry**: Crossplane 2.x images are hosted on `xpkg.crossplane.io`
(not the 1.x `xpkg.upbound.io`). The operator's `RegistryConfig.DefaultRegistry`
defaults to `xpkg.crossplane.io`. Custom mirror configurations must also
account for the new default — package references must be fully qualified with a
registry hostname in 2.x.

**No 1.x migration path**: This operator targets 2.x only. Clusters running
Crossplane 1.x must upgrade to 2.x through the upstream-supported sequential
minor-version path ending at v2.0 before adopting this operator. The operator
does not manage cross-major-version upgrades.

**Removed upstream features**: The following 1.x capabilities no longer exist in
2.x and are not exposed in the CR:
- `ControllerConfig` CRD (replaced by `DeploymentRuntimeConfig`)
- `StoreConfig` / External Secret Stores (dropped entirely)
- Native Patch & Transform composition mode (replaced by function pipelines)
- `writeConnectionSecretToRef` on XRs (compose connection details via functions)
- `deletionPolicy` on namespaced MRs (use `managementPolicies` instead)

**Prometheus metrics**: Crossplane 2.0 renamed metric prefixes
(`crossplane_composition_*` → `engine_*` / `function_*`). The operator's
`ObservabilityConfig` documents the 2.x metric names. If the operator ships
example Grafana dashboards or PrometheusRules, they must use the 2.x names.

## Implementation Plan

### Agents and file ownership

| Agent | Scope | Files owned (write) |
|---|---|---|
| A — Operator core | CRD types, controller, webhooks | `api/`, `internal/controller/`, `cmd/`, `Dockerfile`, `go.mod` |
| B — Helm integration | Chart embedding, rendering, value mappings | `internal/helm/`, `internal/version/`, `charts/`, `hack/sync-upstream.sh` |
| C — OLM packaging | Bundle manifests, FBC catalog, generation scripts | `bundle/`, `catalog/`, `hack/generate-bundle.sh` |
| D — CI/CD | GitHub Actions workflows, Makefile targets | `.github/workflows/`, `Makefile` |

Shared/read-only for all agents: `api/v1alpha1/` types (after Agent A
stabilizes them in phase 1).

### Phased execution (3 phases)

**Phase 1** — CRD types + CI scaffold (parallel: A + D)
- Agent A: define `CrossplaneConfigSpec`, `CrossplaneConfigStatus`, sub-structs,
  run `controller-gen`, stabilize the API surface. Also scaffold the controller
  and webhook stubs.
- Agent D: scaffold workflow files (`ci.yml`, `release.yml`,
  `catalog-publish.yml`, `daily-sync.yml`) with placeholder build targets. Set
  up `Makefile` with standard operator-sdk targets.

**Phase 2** — Helm integration + OLM scaffold (parallel: B + C-scaffold)
- Agent B: vendor upstream charts, implement `renderer.go`, `applier.go`,
  version registry, and all `mapping_v2_X.go` files. Requires Agent A's
  types.
- Agent C (scaffold only): create bundle directory structure,
  `metadata/annotations.yaml`, `bundle.Dockerfile`, and
  `hack/generate-bundle.sh`. Write CRD YAML into bundles (depends on A's
  `controller-gen` output). CSV templates can be stubbed with placeholders for
  Helm-managed resource list.

**Phase 3** — OLM finalization + CI wiring (parallel: C-finalize + D-finalize)
- Agent C (finalize): generate final CSVs using the Helm-managed resource list
  from Agent B's chart rendering. Build FBC catalog fragment with correct
  bundle versions. Run `operator-sdk bundle validate` and `opm validate`.
- Agent D (finalize): wire real build/push targets into workflows now that
  operator image, bundle, and catalog artifacts are defined. Add e2e test job.

### Estimated agent count

4 agents, 3 phases. Maximum 2 agents active in parallel at any point.

## Testing Strategy

### Unit tests

- **Controller**: table-driven tests with a fake client; assert correct Helm
  values are produced for each `CrossplaneConfigSpec` variant; assert status
  conditions are set correctly for error cases.
- **Value mappings**: each `mapping_v2_X.go` tested against a golden
  `values.yaml` snapshot for that version. Include cases for version-specific
  differences (e.g. `AlphaPipelineInspector` ignored on v2.0/v2.1, mapped on
  v2.2; `--max-reconcile-rate` vs `--max-concurrent-reconciles` naming).
- **`extraHelmValues` merge**: test that extraHelmValues overrides a
  conflicting mapped value, that deep-merge preserves non-conflicting mapped
  keys, and that malformed JSON sets a `Degraded` condition rather than
  panicking.
- **Version registry**: test version lookup, unknown-version error path.
- **Webhook**: test singleton enforcement (admit first, reject second).

### Integration tests (envtest)

- Create a `CrossplaneConfig` CR → assert the controller renders and applies
  the expected set of child resources (Deployments, ServiceAccounts, RBAC).
- Update `spec.version` → assert the controller re-renders with the new chart.
- Delete the CR → assert the finalizer removes all child resources.

### E2E tests (KinD + OLM)

- Install the operator via OLM bundle on a KinD cluster.
- Create a `CrossplaneConfig` with default settings → Crossplane pods reach
  Ready within 5 minutes.
- Modify feature flags → assert Crossplane deployment args update.
- Upgrade from v2.0 → v2.1 by changing `spec.version` → assert rollout
  completes without downtime on existing XRs.
- Set `spec.defaultActivations` to a subset → assert only specified MR CRDs
  are installed by the provider.
- OLM channel switch: update Subscription channel from `stable-v2.1` to
  `stable-v2.2`, verify operator bundle upgrades via OLM, then update CR
  `spec.version` to v2.2 and assert Crossplane pods roll out to the new
  version.

### Acceptance criteria

- `CrossplaneConfig` CR creation installs a fully functional Crossplane.
- All config areas from requirements are exposed and reconciled (including
  2.x additions: MRAP, function cache, service account, runtime class).
- OLM bundle installs cleanly via `operator-sdk run bundle`.
- FBC catalog validates with `opm validate`.
- Daily sync detects a simulated upstream chart change and opens a PR.
- Z-stream upgrade within a channel works automatically (e.g. v2.1.0 → v2.1.1
  via `stable-v2.1` with automatic approval).
- Cross-minor upgrade works via channel switch (e.g. `stable-v2.1` →
  `stable-v2.2`).

## Consequences

**What we gain:**

- OpenShift-native lifecycle management for Crossplane with zero Helm knowledge
  required from cluster admins.
- Stable CR API decoupled from upstream Helm values churn — version-specific
  mappings absorb the delta.
- Automated tracking of upstream releases reduces maintenance toil.

**Versioning model**: The operator has its own semver release (Git tags, e.g.
`v0.3.1`) independent of the Crossplane versions it supports. OLM bundle
versions track upstream Crossplane patch versions (e.g.
`crossplane-olm-operator.v2.1.5`), but the operator image tag embedded in the
CSV uses the operator's own version. When an operator-only bug fix ships (no
upstream Crossplane change), we release a new operator version, rebuild all
active bundles with the new operator image reference, bump each bundle's patch
component (e.g. v2.1.5 → v2.1.6 with a release note indicating "operator fix
only"), and update the FBC catalog. This ensures the fix propagates through
OLM's standard upgrade path.

**What we give up / accept:**

- **Lag on new Crossplane features**: New Helm values added upstream won't be
  exposed in the CR until we add them to the mapping. Mitigation: the daily
  sync PR makes these visible quickly, and `spec.extraHelmValues` (see §2)
  provides immediate untyped access to any Helm value.
- **Operator binary size**: Embedding 3 Helm charts increases the image by
  ~15–20 MB. Acceptable for an infrastructure operator.
- **CRD surface area growth**: The CrossplaneConfig CRD will grow as we expose
  more knobs. Mitigation: use sub-structs with clear ownership boundaries; add
  new fields as optional with sane defaults.
- **Single-repo coupling**: A breaking change in CI tooling (e.g. `opm` API
  change) can block operator development. Mitigation: pin tool versions in
  CI and update them in dedicated PRs.
