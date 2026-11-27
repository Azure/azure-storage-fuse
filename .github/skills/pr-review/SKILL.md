---
name: pr-review
description: 'Review a blobfuse2 pull request or diff for data loss, concurrency, errno, security and compatibility problems specific to this FUSE filesystem. Use when asked to review code, a PR, a branch or uncommitted changes in this repository, or to check your own change before opening a PR.'
---

# Review a blobfuse2 change

Report only problems you can tie to a line in the diff with a concrete failure: what the application or user would see, and when. Skip formatting, import order and spelling; `make verify` and CI check them. Order findings by severity.

## 1. Data loss and corruption (highest severity)

- Can `close()`, `fsync()` or `flush()` report success when the data did not reach storage? Upload errors must be returned to the caller, not only logged.
- Does the change alter when `file_cache` or `block_cache` uploads or deletes local data? Check writes served natively by libfuse (dirty mark), several descriptors for one open file (`dup`, `fork`), a flush while another descriptor is still writing, and rename followed by reuse of the old name.
- Can eviction or cleanup remove a file that is still open or being uploaded?

## 2. Concurrency

- Is shared state (maps, handle fields, counters) read and written under the same lock? Look for new goroutines without a stop path in `Stop()`.
- Can a lock be held while waiting on something that needs the same lock: a channel send to a full queue, the eviction worker, or a call down the pipeline that comes back to this component?
- Locks for two paths (rename) are taken source then destination; a new pair taken in the opposite order can deadlock.
- Does the PR add or update a test that runs the concurrent case, and was it run with `-race -count=20`?

## 3. Errors returned to applications

- libfuse callbacks map errors with `errnoOf(err, <errnos>...)`, which matches with `errors.Is` and turns anything not listed into `EIO`. Check that a new failure case either reaches the application as the errno it needs, through a component error that `errors.Is` matches and a callback that lists that errno, or is meant to be `EIO`.
- `GetAttr` returns a not-exist error only for a path that doesn't exist: `attr_cache` caches it as a negative entry, so a transient failure reported as `ENOENT` hides the file until the entry expires.
- New storage error codes are classified in `component/azstorage/utils.go` instead of surfacing as `EIO`.
- Errors aren't swallowed: a returned `nil` after a failed storage call is a bug unless the comment explains why.

## 4. Both variants

- A change in `component/libfuse/libfuse_handler.go` (fuse3) needs the same change in `libfuse2_handler.go` (fuse2), and vice versa.
- A change in `component/azstorage/block_blob.go` usually needs the same change in `datalake.go` for accounts with hierarchical namespace.

## 5. Security

- Nothing logs account keys, SAS tokens, client secrets, bearer tokens or whole auth config structs; no secret-bearing header or query parameter is added to `allowedHeaders` or `allowedQueryParams`.
- Config files that blobfuse2 writes, which can hold credentials, go through `writePrivateConfig` in `cmd/config_files.go` rather than `os.WriteFile` or `os.Create`.
- New code that turns a name into a local path, such as `filepath.Join(cacheDir, name)`, first checks it with `common.IsValidObjectName`, as `file_cache` does in `validateObjectName`, so names with `..` or `\` can't resolve outside the cache directory. In `xload`, file operations go through `xl.root` (an `os.Root`) and names through `localName()`.
- cgo code: C memory never holds Go pointers (open handles are passed to C as their `handlemap.HandleID` and looked up again), C memory is freed exactly once, and buffer sizes from libfuse are respected.
- Cache files and directories aren't created with wider permissions than the existing code uses.

## 6. Compatibility

- A renamed or removed config option, a changed default, or a changed CLI flag is listed under **Breaking Changes** in `CHANGELOG.md`.
- A change to `internal.Component` or an options struct breaks custom `.so` components unless it only adds a field whose zero value keeps the old behavior; see the [change-component-interface skill](../change-component-interface/SKILL.md).

## 7. Performance on hot paths

- `libfuse_read`, `libfuse_write`, `GetAttr` and directory listing run for every application call. Flag new storage requests, allocations per call, or `log.Info` and above on these paths.
- CI compares the libfuse callback benchmarks for changes to `component/libfuse`, `internal/handlemap`, `go.mod` or `go.sum`, and fails on a slowdown of more than 10%. A `[perf-regression-accepted]` line in a commit message overrides it; check that the PR explains the measured cost.

## 8. Completeness

- A bug fix has a test that fails without it; the tests run without Azure credentials (`make test`) and don't use fixed sleeps.
- A user-visible change has a `CHANGELOG.md` entry under the top `(Unreleased)` heading; a new option is in `setup/advancedConfig.yaml`.
- New Go files start with the two-line license header (`// Copyright (c) Microsoft Corporation.`, `// Licensed under the MIT License.`), below any `//go:build` line; `NOTICE` is regenerated with `./notices_fix.sh` when `go.mod` changes.
- The PR description lists the commands that were run and their results.
