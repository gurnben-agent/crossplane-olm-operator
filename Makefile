# Image URL to use all building/pushing image targets
IMG ?= ghcr.io/gurnben-agent/crossplane-olm-operator:latest
BUNDLE_IMG ?= ghcr.io/gurnben-agent/crossplane-olm-operator-bundle:latest
CATALOG_IMG ?= ghcr.io/gurnben-agent/crossplane-olm-operator-catalog:latest

# Tool versions
CONTROLLER_GEN_VERSION ?= v0.17.2
GOLANGCI_LINT_VERSION ?= v1.64.5
OPERATOR_SDK_VERSION ?= v1.39.1
OPM_VERSION ?= v1.52.0
KUSTOMIZE_VERSION ?= v5.6.0

# Tool binaries
LOCALBIN ?= $(shell pwd)/bin
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint
KUSTOMIZE ?= $(LOCALBIN)/kustomize

# Go settings
GOFLAGS ?=
GOTEST_FLAGS ?= -race -count=1

# Platforms for multi-arch builds
PLATFORMS ?= linux/amd64,linux/arm64

.PHONY: all
all: build ## Default target: build the operator binary

##@ General

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)

##@ Development

.PHONY: generate
generate: controller-gen ## Generate DeepCopy and RBAC manifests
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: manifests
manifests: controller-gen ## Generate CRD and webhook manifests
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: fmt
fmt: ## Run go fmt
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: lint
lint: golangci-lint ## Run golangci-lint
	$(GOLANGCI_LINT) run ./...

.PHONY: test
test: generate manifests fmt vet ## Run unit tests
	go test $(GOTEST_FLAGS) ./...

.PHONY: test-unit
test-unit: ## Run unit tests only (no codegen)
	go test $(GOTEST_FLAGS) ./...

##@ Build

.PHONY: build
build: generate fmt vet ## Build the operator binary
	go build $(GOFLAGS) -o bin/manager cmd/main.go

.PHONY: run
run: generate manifests fmt vet ## Run the operator locally against the configured cluster
	go run ./cmd/main.go

.PHONY: docker-build
docker-build: ## Build the operator container image
	docker build -t $(IMG) .

.PHONY: docker-push
docker-push: ## Push the operator container image
	docker push $(IMG)

.PHONY: docker-buildx
docker-buildx: ## Build and push multi-arch operator image
	docker buildx build --platform $(PLATFORMS) --push -t $(IMG) .

##@ Bundle

.PHONY: bundle
bundle: manifests kustomize ## Generate OLM bundle manifests
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION)
	operator-sdk bundle validate ./bundle

.PHONY: bundle-generate
bundle-generate: manifests ## Regenerate bundle manifests from CRD output
	bash hack/generate-bundle.sh

.PHONY: bundle-build
bundle-build: ## Build OLM bundle images for all versions
	@for ver in v2.0 v2.1 v2.2; do \
		echo "Building bundle image for $${ver}..."; \
		docker build -f bundle/$${ver}/bundle.Dockerfile -t $(BUNDLE_IMG)-$${ver} bundle/$${ver}; \
	done

.PHONY: bundle-push
bundle-push: ## Push OLM bundle images for all versions
	@for ver in v2.0 v2.1 v2.2; do \
		echo "Pushing bundle image for $${ver}..."; \
		docker push $(BUNDLE_IMG)-$${ver}; \
	done

.PHONY: bundle-validate
bundle-validate: ## Validate all OLM bundles
	@for ver in v2.0 v2.1 v2.2; do \
		echo "Validating bundle $${ver}..."; \
		operator-sdk bundle validate ./bundle/$${ver}; \
	done

##@ Catalog

.PHONY: catalog-build
catalog-build: ## Build the FBC catalog image
	docker build -f catalog.Dockerfile -t $(CATALOG_IMG) .

.PHONY: catalog-push
catalog-push: ## Push the FBC catalog image
	docker push $(CATALOG_IMG)

.PHONY: catalog-validate
catalog-validate: ## Validate the FBC catalog
	opm validate catalog/

##@ E2E

.PHONY: e2e
e2e: docker-build ## Run e2e tests against a KinD cluster with OLM
	@echo "Creating KinD cluster..."
	kind create cluster --name e2e --wait 120s || true
	kind load docker-image $(IMG) --name e2e
	@echo "Installing OLM..."
	operator-sdk olm install || true
	@echo "Building and loading bundle image..."
	docker build -f bundle/v2.2/bundle.Dockerfile -t crossplane-olm-operator-bundle:e2e bundle/v2.2
	kind load docker-image crossplane-olm-operator-bundle:e2e --name e2e
	@echo "Running bundle..."
	operator-sdk run bundle crossplane-olm-operator-bundle:e2e --timeout 5m
	@echo "E2E setup complete — apply a CrossplaneConfig CR to test."

.PHONY: e2e-cleanup
e2e-cleanup: ## Delete the KinD e2e cluster
	kind delete cluster --name e2e

##@ Sync

.PHONY: sync-upstream
sync-upstream: ## Sync upstream Crossplane Helm charts
	bash hack/sync-upstream.sh

##@ Tool dependencies

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

.PHONY: controller-gen
controller-gen: $(LOCALBIN) ## Install controller-gen
	@test -s $(CONTROLLER_GEN) && $(CONTROLLER_GEN) --version | grep -q $(CONTROLLER_GEN_VERSION) || \
		GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)

.PHONY: golangci-lint
golangci-lint: $(LOCALBIN) ## Install golangci-lint
	@test -s $(GOLANGCI_LINT) && $(GOLANGCI_LINT) --version | grep -q $(GOLANGCI_LINT_VERSION) || \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(LOCALBIN) $(GOLANGCI_LINT_VERSION)

.PHONY: kustomize
kustomize: $(LOCALBIN) ## Install kustomize
	@test -s $(KUSTOMIZE) && $(KUSTOMIZE) version | grep -q $(KUSTOMIZE_VERSION) || \
		GOBIN=$(LOCALBIN) go install sigs.k8s.io/kustomize/kustomize/v5@$(KUSTOMIZE_VERSION)
