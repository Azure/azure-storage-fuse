# Local entry points for developers and coding agents.
# Each target mirrors a check in blobfuse2-1es_ci.yaml, so a passing `make verify` should mean a
# passing CI run, except for tests that need Azure Storage credentials (see `make test-azure`).

SHELL := /bin/bash
.DEFAULT_GOAL := help

# fuse3 (default) or fuse2
FUSE ?= fuse3

# Same toolchain settings as build.sh.
export CGO_ENABLED := 1
export GOTOOLCHAIN := local

# Keep in sync with the "Static Analysis (Lint)" step in blobfuse2-1es_ci.yaml.
GOLANGCI_LINT_VERSION ?= v2.11.0
BIN_DIR := $(CURDIR)/.bin
GOLANGCI_LINT := $(BIN_DIR)/golangci-lint-$(GOLANGCI_LINT_VERSION)
# Without a soft memory limit, a lint run with an empty cache peaks near 10 GB and can be OOM-killed.
LINT_MEMLIMIT ?= 3GiB

# Keep in sync with the "libfuse callback benchmarks vs base" step in blobfuse2-1es_ci.yaml. The compare script runs
# benchstat from PATH, so each version gets its own directory.
BENCHSTAT_VERSION ?= v0.0.0-20261009192801-be2c69fb417e
BENCHSTAT_DIR := $(BIN_DIR)/benchstat-$(BENCHSTAT_VERSION)
# Revision whose merge base with HEAD bench-libfuse compares against
BASE ?= main

# Unit-test suites that use a real storage account from ~/azuretest.json.
AZURE_SUITES := ^(TestAuthTestSuite|TestBlockBlob|TestDatalake)$$

# Packages that hold timing- and resource-sensitive suites fail intermittently when go test runs
# them in parallel with each other, so run test packages one at a time by default.
TEST_PARALLEL ?= 1
TEST_FLAGS ?=
PKG ?= ./...

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: build
build: ## Build ./blobfuse2 (FUSE=fuse2 for the libfuse2 variant)
	./build.sh $(if $(filter fuse2,$(FUSE)),fuse2,)

.PHONY: build-health
build-health: ## Build the ./bfusemon health monitor
	./build.sh health

.PHONY: tools
tools: $(GOLANGCI_LINT) $(BENCHSTAT_DIR)/benchstat ## Install the pinned golangci-lint and benchstat into .bin/

$(GOLANGCI_LINT):
	@mkdir -p $(BIN_DIR)
	curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b $(BIN_DIR) $(GOLANGCI_LINT_VERSION)
	@mv $(BIN_DIR)/golangci-lint $@

$(BENCHSTAT_DIR)/benchstat:
	GOBIN=$(BENCHSTAT_DIR) go install golang.org/x/perf/cmd/benchstat@$(BENCHSTAT_VERSION)

.PHONY: fmt
fmt: $(GOLANGCI_LINT) ## Format Go files in place (gofmt -s and goimports)
	$(GOLANGCI_LINT) fmt

.PHONY: fmt-check
fmt-check: ## Fail if any Go file is not gofmt -s formatted
	@out=$$(gofmt -s -l .); if [ -n "$$out" ]; then echo "Not gofmt -s formatted (run make fmt):"; echo "$$out"; exit 1; fi

.PHONY: lint
lint: $(GOLANGCI_LINT) ## Run the pinned golangci-lint with .golangci.yml
	GOMEMLIMIT=$(LINT_MEMLIMIT) $(GOLANGCI_LINT) run --build-tags $(FUSE)

.PHONY: copyright-check
copyright-check: ## Fail if a tracked Go, C or template file lacks the license header (git add new files first)
	python3 -m unittest discover -s test/scripts -p 'test_check_copyright.py'
	python3 test/scripts/check_copyright.py

.PHONY: check
check: ## Compile all packages and tests for fuse3 and fuse2 without running tests (fast)
	@set -o pipefail; for fuse in fuse3 fuse2; do echo "go test -tags=unittest,$$fuse -run '^$$' ./..."; go test -tags=unittest,$$fuse -count=1 -run '^$$' ./... 2>&1 | { grep -vE '^(ok|\?) ' || true; } || exit 1; done

.PHONY: test
test: ## Run unit tests that need no Azure credentials (PKG=./component/file_cache to narrow)
	go test -p $(TEST_PARALLEL) -count=1 -timeout=30m -tags=unittest,$(FUSE) -skip '$(AZURE_SUITES)' $(TEST_FLAGS) $(PKG)

.PHONY: test-race
test-race: ## Run unit tests with the race detector (PKG=./component/file_cache to narrow)
	go test -race -p $(TEST_PARALLEL) -count=1 -timeout=30m -tags=unittest,$(FUSE) -skip '$(AZURE_SUITES)' $(TEST_FLAGS) $(PKG)

.PHONY: test-loopback
test-loopback: build ## Mount blobfuse2 over a local directory with FUSE and run the loopback E2E test
	go test -count=1 -timeout=15m ./test/loopback_e2e -args -blobfuse2=$(CURDIR)/blobfuse2

.PHONY: test-integration
test-integration: ## Run the libfuse tests that mount FUSE in-process over a Go backend (not run by CI)
	go test -count=1 -timeout=15m -tags=integration $(TEST_FLAGS) ./component/libfuse

.PHONY: test-azure
test-azure: ## Run the azstorage suites against the real account in ~/azuretest.json
	@test -f $(HOME)/azuretest.json || { echo "$(HOME)/azuretest.json not found; these suites need a storage account."; exit 1; }
	go test -count=1 -timeout=45m -tags=unittest,$(FUSE) -run '$(AZURE_SUITES)' $(TEST_FLAGS) ./component/azstorage

.PHONY: bench-libfuse
bench-libfuse: $(BENCHSTAT_DIR)/benchstat ## Compare the libfuse callback benchmarks of the working tree with BASE (default main)
	PATH="$(BENCHSTAT_DIR):$$PATH" test/scripts/libfuse_bench_compare.sh "$$(git merge-base HEAD $(BASE))"

.PHONY: verify
verify: fmt-check copyright-check check lint build test ## Run the checks to pass before opening a PR
