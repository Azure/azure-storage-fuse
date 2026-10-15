# AGENTS.md

Blobfuse2 is a FUSE filesystem for Linux, written in Go with cgo, that mounts Azure Blob Storage and ADLS Gen2 containers. User documentation lives on Microsoft Learn (linked from [README.md](README.md)). This file is for people and coding agents who change the code.

## Commands

Run commands from the repository root. You need the Go version in `go.mod`, `gcc`, `libfuse3-dev`, `fuse3`, and `libfuse-dev` (the fuse2 headers that `make check` and `FUSE=fuse2` use). The Makefile sets `GOTOOLCHAIN=local`, so Go is never downloaded automatically.

| Task | Command | Notes |
|---|---|---|
| Build | `make build` | Writes `./blobfuse2`. `make build FUSE=fuse2` builds the libfuse2 variant. |
| Compile check | `make check` | Compiles all packages and tests for fuse3 and fuse2 without running them; about 30 seconds. Use it as the quick check while you edit. |
| Unit tests | `make test` | No Azure credentials needed; about 6 minutes. Narrow with `PKG=./component/file_cache TEST_FLAGS='-run TestFileCacheTestSuite/TestOpenFile -v'`. |
| Race detector | `make test-race PKG=./component/file_cache` | Run for any change to locking, goroutines or handle lifetime. |
| Real FUSE mount, no Azure | `make test-loopback` | Mounts over a local directory with `loopbackfs`. Needs `/dev/fuse` and `fusermount3`. |
| libfuse FUSE integration tests | `make test-integration` | Mounts FUSE in-process over a Go backend (`-tags=integration`); about 10 seconds. Needs `/dev/fuse` and `user_allow_other` in `/etc/fuse.conf` or root. CI doesn't run these. |
| Lint | `make lint` | golangci-lint pinned to the CI version. Other versions report different findings. |
| Format | `make fmt`, `make fmt-check` | |
| License headers | `make copyright-check` | Runs the CI checker, `test/scripts/check_copyright.py`, which checks only files tracked by git: `git add` new files first. |
| libfuse benchmarks | `make bench-libfuse` | Compares the libfuse callback benchmarks of your working tree with its merge base with `main` (`BASE=<ref>` to change it); about 4 minutes, on an otherwise idle machine. CI runs the same comparison for changes to `component/libfuse`, `internal/handlemap`, `go.mod` or `go.sum`, and fails on a slowdown of more than 10%. |
| Before a PR | `make verify` | `fmt-check`, `copyright-check`, `check`, `lint`, `build` and `test`; about 8 minutes. |
| Azure-backed tests | `make test-azure` | Uses the storage account in `~/azuretest.json` and creates and deletes containers. Run only when asked. |

When you call `go test` directly, pass `-tags=unittest,fuse3` (or `unittest,fuse2`). Without `unittest`, `go test ./...` also builds the E2E and mount tests (`//go:build !unittest`), which expect a mounted container.

## Architecture

A file operation goes from the kernel through libfuse callbacks into a pipeline of components and ends in storage:

`kernel → libfuse → xload | block_cache | file_cache → attr_cache → azstorage → Azure Storage`

- Every component implements `internal.Component` in [internal/component.go](internal/component.go) and embeds `internal.BaseComponent`, which forwards each call to `NextComponent()`. Override only the operations that the component changes.
- The `components:` list in the config file sets the order. [internal/pipeline.go](internal/pipeline.go) rejects a config in which a component's priority is higher than that of the component above it: `libfuse` (1000) > `xload`, `block_cache`, `file_cache`, `distributed_cache`, `entry_cache` (500) > `attr_cache` (300) > `azstorage`, `loopbackfs` (100). `stream` is an alias for `block_cache`.
- `cmd/mount.go` adjusts the list before the pipeline is built. `file_cache`, `block_cache` and `xload` are mutually exclusive (`common.ValidatePipeline`), and `entry_cache` is inserted after `libfuse` when it is configured. `distributed_cache` is a second-level cache for read-only mounts: users list it without another cache, and mount inserts `block_cache` above it.
- Components register themselves in `init()` with `internal.AddComponent`, and the blank imports in [cmd/imports.go](cmd/imports.go) link them into the binary.

| Path | Contents |
|---|---|
| `cmd/` | Cobra CLI: `mount`, `unmount`, `gen-config`, `secure`, `health-monitor`, and the hidden `doc` command that regenerates `doc/` |
| `common/` | Shared types and helpers; `config/` (process-global Viper wrapper), `log/`, `cache_policy/` |
| `internal/` | Component interface, `BaseComponent`, pipeline, `handlemap` (open file handles), `stats_manager` |
| `component/libfuse/` | cgo glue to libfuse: `libfuse_handler.go` (fuse3) and `libfuse2_handler.go` (fuse2) |
| `component/file_cache/` | Whole-file cache on local disk ("caching" mode) |
| `component/block_cache/` | Block reads and writes with prefetching ("streaming" mode) |
| `component/attr_cache/`, `component/entry_cache/` | Attribute and directory-listing caches |
| `component/xload/` | Preloads a container or directory into the local cache |
| `component/distributed_cache/` | Second-level read cache on a DACS cache server cluster, which reduces Azure egress and read latency for multi-node workloads; see [doc/blobfuse2-dacs-getting-started.md](doc/blobfuse2-dacs-getting-started.md) and [sampleDistCacheConfig.yaml](sampleDistCacheConfig.yaml) |
| `component/azstorage/` | Azure SDK calls; `block_blob.go` (flat namespace) and `datalake.go` (hierarchical namespace) both implement `AzConnection` |
| `component/loopback/` | `loopbackfs`, which uses a local directory as storage; used by tests |
| `component/custom/` | Loads external components from `.so` plugins |
| `tools/health-monitor/` | The `bfusemon` health monitor |
| `test/` | E2E, mount, stress and loopback tests; `dcache_e2e` tests `distributed_cache` in a kind cluster with DACS (set up by `test/scripts/dcache/`) and a storage account |
| `setup/` | `baseConfig.yaml` (short sample), `advancedConfig.yaml` (reference for every option), packaging files |

Azure Pipelines is the CI of record ([blobfuse2-1es_ci.yaml](blobfuse2-1es_ci.yaml), [azure-pipeline-templates/](azure-pipeline-templates)). It runs the unit tests with storage credentials, lint, format, license-header and `NOTICE` checks, and the libfuse benchmark comparison. GitHub Actions run CodeQL, codespell, Trivy and an ARM32 build.

## Rules

- **Errors**: components return Go errors, and each libfuse callback turns them into the negative errno it reports with `errnoOf` in [component/libfuse/errno.go](component/libfuse/errno.go). It matches with `errors.Is`, so wrapped and joined errors work, and counts `fs.ErrNotExist`, `fs.ErrExist` and `fs.ErrPermission` as `ENOENT`, `EEXIST` and `EACCES`; an error that matches none of the errnos a callback reports becomes `EIO`. `CreateFile` must return an error matching `fs.ErrExist` when the file exists. `GetAttr` returns a not-exist error only for a path that doesn't exist, because `attr_cache` caches that as a negative entry; caches pass other `GetAttr` errors through unchanged.
- **fuse3 and fuse2**: a change to `libfuse_handler.go` usually needs the same change in `libfuse2_handler.go` and in the matching `*_test_wrapper.go`. `make check` compiles both variants; also run `make test FUSE=fuse2 PKG=./component/libfuse`.
- **Block blob and ADLS**: a behavior change in `azstorage` usually needs the same change in both `block_blob.go` and `datalake.go`.
- **Logging**: use `common/log` (`log.Trace`, `Debug`, `Info`, `Warn`, `Err`, `Crit`) with the existing `Component::Method : message [%s]` format.
- **Secrets**: never log account keys, SAS tokens, client secrets or whole auth configs. Config files that blobfuse2 writes can hold credentials, so write them with `writePrivateConfig` in [cmd/config_files.go](cmd/config_files.go), which creates them with mode 0600, replaces them atomically and refuses directories that other users can modify.
- **License headers**: every Go file and `internal/component.template` starts with `// Copyright (c) Microsoft Corporation.` and `// Licensed under the MIT License.`, below the `//go:build` line and a blank line if the file has a build constraint, as in `component/libfuse/libfuse_handler.go`. C files use the block-comment form at the top of `component/libfuse/native_file_io.h`.
- **Global state**: config is a process-global Viper instance (`config.ResetConfig()` clears it), and libfuse keeps the pipeline head in the global `fuseFS`. Tests that use them can't run in parallel.
- **Mocks**: `internal/mock_component.go` is a gomock mock (`github.com/golang/mock` v1.6.0) that is maintained by hand and has extra behavior, for example in `RenameFile`. Add or change methods in place; don't regenerate the file.
- **Dependencies**: after changing `go.mod`, run `./notices_fix.sh` (it downloads license texts) and commit `NOTICE`. CI fails when `NOTICE` is stale.
- **libfuse performance**: CI fails a change to `component/libfuse`, `internal/handlemap`, `go.mod` or `go.sum` that slows a libfuse callback benchmark by more than 10%; check with `make bench-libfuse`. If a change accepts a measured slowdown on purpose, put `[perf-regression-accepted]` on a line of its own in a commit message and explain the numbers in the PR.
- **Changelog**: a change that users can see needs an entry under the top `## x.y.z (Unreleased)` heading of [CHANGELOG.md](CHANGELOG.md), in **Breaking Changes**, **Features** or **Bug Fixes**. Describe what users saw and when it happened, and end with `([PR #N](https://github.com/Azure/azure-storage-fuse/pull/N))`.
- **Config options**: follow the [add-config-option skill](.github/skills/add-config-option/SKILL.md). An option touches the component's options struct, its CLI flag, `GenConfig`, `setup/advancedConfig.yaml` and the changelog.

## Testing

- Tests use testify suites. Set the logger to `silent` in `SetupTest`, create paths with `t.TempDir()`, and give each test its own config string.
- Cache components are tested with `loopbackfs` or `internal.MockComponent` as the next component. `azstorage` behavior is tested offline with `httptest.Server` fakes, as in `component/azstorage/datalake_create_test.go`, not against a real account.
- A bug fix needs a test that fails without the fix. For races and hangs, also run the test with `-race -count=20`, like the file-cache stress step in `blobfuse2-1es_ci.yaml`.
- `go test` runs packages in parallel by default, and the `file_cache`, `block_cache` and `cmd` suites then fail intermittently on small machines. That is why `make test` uses `-p 1`.

## Definition of done

1. `make verify` passes, and so does `make bench-libfuse` if you changed `component/libfuse`, `internal/handlemap`, `go.mod` or `go.sum`. If a step can't run in your environment, such as the loopback test without `/dev/fuse`, say so in the PR.
2. New or changed behavior has unit tests. Behavior that users see through the mount is also checked with `make test-loopback`, `make test-integration` (libfuse) or the [loopback-mount skill](.github/skills/loopback-mount/SKILL.md).
3. `CHANGELOG.md` and `setup/advancedConfig.yaml` are updated when users can see the change.
4. The PR description follows [.github/pull_request_template.md](.github/pull_request_template.md) and lists each command that was run with its result.
