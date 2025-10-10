# Repository Guidelines

## Project Structure & Module Organization
- `cmd/`: CLI entry points (Cobra commands).
- `component/`: Core FUSE, cache, and storage components (`libfuse/`, `block_cache/`, `file_cache/`, `azstorage/`, `attr_cache/`).
- `common/` and `internal/`: Shared utilities, config, logging, and pipeline.
- `test/`: Unit, E2E, perf, and helper scripts; `testdata/` holds fixtures.
- `setup/`, `doc/`, `docker/`: Config templates, docs, and container tooling.
- Root: `build.sh`, `main.go`, `.golangci.yml`, samples like `sampleFileCacheConfig.yaml`.

## Build, Test, and Development Commands
- Build (fuse3 default): `./build.sh`
- Build (fuse2): `./build.sh fuse2`
- Health monitor: `./build.sh health`
- Lint: `$(go env GOPATH)/bin/golangci-lint run --tests=false --build-tags fuse3`
- Core unit tests: `go test -v -timeout=10m ./internal/... ./common/... --tags=unittest,fuse3`
- Full tests: `go test -v -timeout=45m ./... --tags=unittest,fuse3`
- Quick binary check: `./blobfuse2 --version`

## Coding Style & Naming Conventions
- Language: Go. Format with `gofmt -s`; CI uses `golangci-lint` (see `.golangci.yml`).
- Indentation and imports follow `gofmt`/`goimports` defaults.
- Packages: lowercase, no underscores; exported identifiers use PascalCase, unexported use camelCase.
- Filenames: `snake_case.go`; tests end with `_test.go`.
- Prefer small, composable packages under `component/` and keep CLI logic in `cmd/`.

## Testing Guidelines
- Frameworks: Go `testing` + `testify` assertions.
- Tags: many tests expect `--tags=unittest,fuse3`. E2E and mount tests require Azure credentials and Linux FUSE.
- Naming: `TestXxx(t *testing.T)`; table-driven where sensible. Group by package.
- Aim not to reduce existing coverage; add tests when changing core logic.

## Commit & Pull Request Guidelines
- Commits: short imperative subject; reference PR/issue when applicable (e.g., "Fix cache eviction (#1234)").
- PRs must include: clear description, rationale, test plan/outputs, and docs updates (README/config samples) when user‑facing.
- Lint and tests must pass; avoid introducing new `golangci-lint` violations.

## Security & Configuration Tips
- Never commit secrets; prefer env vars (e.g., `AZURE_STORAGE_SAS_TOKEN`, `AZURE_STORAGE_ACCOUNT`).
- Validate builds on Linux with FUSE3 (`libfuse3-dev`). Use build tags `fuse3`/`fuse2` as needed.
