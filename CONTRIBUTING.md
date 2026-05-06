# Contributing to crossplane-olm-operator

## Development Setup

1. Install Go 1.23+
2. Clone the repository:
   ```bash
   git clone https://github.com/gurnben-agent/crossplane-olm-operator.git
   cd crossplane-olm-operator
   ```
3. Build:
   ```bash
   make build
   ```

## Making Changes

1. Create a feature branch from `main`.
2. Make your changes.
3. If you modified types in `api/v1alpha1/`, run:
   ```bash
   make manifests generate
   ```
4. Run tests:
   ```bash
   make test
   ```
5. Commit with a descriptive message following [Conventional Commits](https://www.conventionalcommits.org/) (e.g., `feat:`, `fix:`, `chore:`).
6. Open a pull request against `main`.

## Code Quality

- Run `make lint` before submitting.
- Run `make fmt` to format code.
- All CI checks must pass: lint, vet, unit tests, controller-gen check, bundle validation, FBC validation, and E2E.

## Testing

- **Unit tests**: `make test-unit`
- **E2E tests**: `make e2e` (requires Docker)
- Add tests for new functionality. Controller tests use table-driven style.

## Adding a New Crossplane Version

1. Vendor the chart into `charts/v2.X/` (or use `hack/sync-upstream.sh`).
2. Create `internal/version/mapping_v2_X.go` with the value mapping function.
3. Register the version in `internal/version/registry.go`.
4. Add OLM bundle under `bundle/v2.X/`.
5. Update the FBC catalog in `catalog/crossplane-olm-operator/catalog.yaml`.
6. Add the version to the CI bundle-validate matrix in `.github/workflows/ci.yml`.

## Reporting Issues

Open an issue on the GitHub repository with steps to reproduce.
