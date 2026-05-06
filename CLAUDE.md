# CLAUDE.md

## Project

crossplane-olm-operator — a Kubernetes operator managing Crossplane 2.x on OpenShift via OLM. Built with operator-sdk (Go plugin), kubebuilder v4 layout.

## Build & Test

```bash
make build         # Build binary to bin/manager
make test          # Full test suite (codegen + lint + vet + unit tests)
make test-unit     # Unit tests only: go test -race -count=1 ./...
make lint          # golangci-lint (requires: make golangci-lint to install)
make vet           # go vet ./...
make fmt           # go fmt ./...
make manifests     # Regenerate CRD/webhook/RBAC YAML via controller-gen
make generate      # Regenerate DeepCopy methods
make docker-build  # Build container image (IMG=ghcr.io/gurnben-agent/crossplane-olm-operator:latest)
make e2e           # E2E on KinD: creates cluster, installs OLM, deploys bundle
```

Single test: `go test -race -count=1 -run TestName ./internal/controller/...`

## Key Architecture

- **Single CRD**: `CrossplaneConfig` (cluster-scoped, singleton enforced by webhook)
- **Reconciler**: fetches CR → looks up version registry → renders embedded Helm chart → server-side applies manifests
- **Version mappings**: `internal/version/mapping_v2_X.go` — each file translates stable CR API to version-specific Helm values
- **Charts embedded**: vendored in `charts/<version>/` via `go:embed`, no runtime downloads
- **OLM packaging**: per-version bundles in `bundle/`, FBC catalog in `catalog/`

## Code Layout

- `api/v1alpha1/` — CRD types, webhook (singleton validation)
- `internal/controller/` — reconciler, interfaces (VersionRegistry, ChartRenderer, ManifestApplier)
- `internal/helm/` — renderer.go (Helm SDK chart render), applier.go (server-side apply)
- `internal/version/` — registry.go + mapping_v2_0/1/2.go
- `cmd/main.go` — entrypoint, wires manager
- `config/` — kustomize bases for CRD, RBAC, webhook
- `hack/` — sync-upstream.sh (daily chart sync), generate-bundle.sh

## Conventions

- Go module: `github.com/gurnben-agent/crossplane-olm-operator`
- API group: `crossplane.io/v1alpha1`
- Generated files must be committed (CI checks via `make manifests generate && git diff`)
- Tests use table-driven style with testify or standard testing
- Makefile pins tool versions (controller-gen, golangci-lint, operator-sdk, opm, kustomize)
