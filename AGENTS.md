# AGENTS.md

Guidelines for AI agents working on this repository.

## Getting Started

1. Read `CLAUDE.md` for build commands, architecture, and conventions.
2. Read `docs/adr/0001-olm-operator-for-crossplane.md` for design context.
3. Run `make test-unit` to verify the codebase compiles and tests pass before making changes.

## File Ownership

| Area | Key Files | Notes |
|------|-----------|-------|
| CRD & API | `api/v1alpha1/` | Run `make manifests generate` after type changes |
| Controller | `internal/controller/` | Interfaces: VersionRegistry, ChartRenderer, ManifestApplier |
| Helm integration | `internal/helm/`, `internal/version/` | One mapping file per Crossplane minor version |
| OLM bundles | `bundle/`, `catalog/` | Regenerate via `hack/generate-bundle.sh` |
| CI/CD | `.github/workflows/`, `Makefile` | 4 workflows: ci, release, catalog-publish, daily-sync |
| Charts | `charts/` | Vendored upstream — update via `hack/sync-upstream.sh` |

## Workflow

- After modifying types in `api/v1alpha1/`, always run `make manifests generate` and commit the generated output.
- After modifying value mappings, run `make test-unit` to validate against golden snapshots.
- Bundle and catalog changes should be validated with `operator-sdk bundle validate` and `opm validate catalog/`.

## Testing

- Unit tests: `make test-unit` or `go test -race -count=1 ./...`
- E2E tests: `make e2e` (requires Docker, creates a KinD cluster)
- CI runs lint, vet, unit tests, controller-gen check, bundle validation, FBC validation, and E2E.

## Version Support

The operator tracks Crossplane v2.0, v2.1, and v2.2. When adding support for a new minor version:

1. Add the chart to `charts/v2.X/`
2. Create `internal/version/mapping_v2_X.go` with the value mapping function
3. Register the version in `internal/version/registry.go`
4. Add a new bundle directory under `bundle/v2.X/`
5. Update the FBC catalog in `catalog/crossplane-olm-operator/catalog.yaml`
