# Cache Invalidation Enhancements

This document describes the recent additions that tighten cache invalidation across the attribute and file cache components. It covers design rationale, implementation details, and guidance for exercising the accompanying tests.

## Overview

Two related areas were addressed:

1. **Attribute cache stability** – prevent races while multiple goroutines mutate cached entries and expose explicit helpers for external invalidation requests.
2. **File cache safety** – ensure that directory invalidation honors per-file locks and provide public hooks that the CLI can call to purge items deterministically.

These changes pair with new unit tests that document the intended behavior and serve as guardrails during future refactors.

## Attribute Cache Changes

### Per-entry locking

Historically the attribute cache relied on a coarse `sync.RWMutex` around the map of cached paths. Individual entries (`attrCacheItem`) were updated without additional synchronization. Concurrent invalidation (e.g., CLI invalidate) alongside read operations could mutate the same item simultaneously, causing data races.

To address this:

- Each `attrCacheItem` now carries its own `sync.RWMutex`.
- Mutating helpers (`markDeleted`, `invalidate`, `setSize`, `setMode`) and read helpers (`getAttr`, `isFresh`, `expired`) acquire the per-entry lock before touching cached state.
- Callers continue to hold the outer map lock, preserving pointer stability and avoiding allocation churn.

A short header comment in `component/attr_cache/cacheMap.go` explains why the inner lock exists so that future upstream rewrites can make informed decisions.

### External invalidation helpers

The new CLI command `blobfuse2 mount invalidate` needs to invalidate entries without duplicating logic. Two thin wrappers – `InvalidatePathExt` and `InvalidateDirExt` – now expose the internal `invalidatePath` and `invalidateDirectory` routines.

A corresponding test (`TestInvalidatePathExt`, `TestInvalidateDirExt`) verifies that the helpers pick up existing cache entries and that scoping still matches the existing prefix rules.

### Freshness helpers

Cache expiry decisions now rely on `attrCacheItem.isFresh()` and `attrCacheItem.expired()`. Consolidating the timeout comparison has two benefits:

- The per-item lock is always held while reading `cachedAt`.
- Future changes to timeout semantics (e.g., monotonic timestamps) can be handled in one location.

`TestAttrCacheItemFreshness` exercises the helper across non-zero and zero timeout values.

## File Cache Changes

### Lock-aware directory invalidation

`invalidateDirectory` used to walk local directories and purge files without honoring per-file locks. If an application still had a handle open, the purge could race with I/O.

The walker now:

- Acquires the appropriate file lock via `fc.fileLocks.Get(relPath)` before deleting a file.
- Releases the lock after `CachePurge` and `deleteFile` finish.

Directories themselves are still removed eagerly, matching prior behavior. A new test (`TestInvalidateDirectoryWaitsForLocks`) creates a scenario where a lock is held while invalidation runs; the test asserts that the walker pauses until the lock is released.

### Explicit public helpers

Two exported methods complete the CLI integration:

- `InvalidateFile(name string)` – locks the target, deletes the local copy, and purges the LRU entry.
- `InvalidateDirExt(name string)` – launches `invalidateDirectory` asynchronously, mirroring existing background cleanup behavior.

`TestInvalidateFileRemovesLocalCopy` covers the single-file case to ensure both the filesystem view and the eviction policy are updated.

## Testing

### Unit test coverage

| Test | File | Purpose |
|------|------|---------|
| `TestInvalidatePathExt`, `TestInvalidateDirExt` | `component/attr_cache/attr_cache_test.go` | Validate that external helpers mark cached entries invalid while respecting prefix rules. |
| `TestAttrCacheItemFreshness` | `component/attr_cache/attr_cache_test.go` | Exercise the new freshness helpers with zero and non-zero timeouts. |
| `TestInvalidateFileRemovesLocalCopy` | `component/file_cache/file_cache_test.go` | Ensure that a manual invalidation deletes the local copy and purges the policy. |
| `TestInvalidateDirectoryWaitsForLocks` | `component/file_cache/file_cache_test.go` | Confirm directory invalidation waits for outstanding file locks before deleting. |

### CLI usage

The functionality is surfaced through the `blobfuse2 mount invalidate` subcommand:

```bash
blobfuse2 mount invalidate <mountpoint> <relative-path...> \
    [--scope attr|file|block|all] [--recursive]
```

- `<mountpoint>` must point to a currently mounted blobfuse2 instance; the command resolves the owning process and writes a request under `~/.blobfuse2/ctrl/<pid>`.
- `<relative-path>` entries are relative to the mount root; multiple paths can be listed.
- `--scope` is a single-choice enum. Pick one of `attr`, `file`, `block`, or `all`.  
  `all` invalidates attribute, file, and block caches in one shot. Mixed combinations such as `attr+file` are intentionally not supported to keep the interface deterministic—allowing arbitrary unions would complicate parsing, help text, and error handling. If a partial combination is needed, invoke the command twice with the desired scopes.
- `--recursive` applies to directory paths and recursively invalidates children. Omit it to only invalidate the directory entry itself.

The command writes a JSON request and attempts to send `SIGUSR1`. Even if the signal is blocked, the background scanner will consume the request on its next pass.

### Running the tests

Use the standard Go tooling from the repository root:

```bash
go test ./component/attr_cache ./component/file_cache
```

> **Note:** macOS builds currently fail because `syscall.Statfs_t` does not expose `Frsize`. Run the suite on Linux (or inside a Linux container) where the project normally targets FUSE workloads.

### Adding new scenarios

When extending the cache invalidation logic:

1. Prefer adding tests alongside the existing suites so concurrency expectations remain clear.
2. If new helpers are introduced, add table-driven tests that focus on input/output behavior rather than implementation details.
3. Keep tests deterministic: avoid sleeping arbitrarily unless waiting for the asynchronous eviction goroutine, and use short timeouts to keep the suite fast.

### Invalidate request flow

For completeness, the runtime flow looks like this:

1. `blobfuse2 mount invalidate` resolves the target mount process (`pidof blobfuse2` + command-line match).
2. A JSON request is written to `~/.blobfuse2/ctrl/<pid>/invalidate-<timestamp>-<random>.json`. Writing via a temporary file plus `rename` ensures atomic publication.
3. The CLI sends `SIGUSR1`. If signals are not available, the background ticker still discovers the request.
4. `mount`’s signal handler (and the periodic scanner) calls `processOutstandingInvalidateRequests`, which:
   - Verifies the request scope.
   - Walks the pipeline to locate `attr_cache`, `file_cache`, and `block_cache` components.
   - Invokes the relevant `Invalidate*` helper(s).
5. After processing, the request file is deleted so subsequent scans stay clean.

Although this change set focuses on attribute and file caches, the same mechanism triggers `block_cache.InvalidateFile/InvalidateDirExt` when `scope` includes `block` or `all`, ensuring the three caches remain aligned.

## Summary

The enhancements keep cache invalidation predictable, even under concurrent load, while staying close to the original structure for minimal rebase friction. The tests both verify the new behavior and act as living documentation for the assumptions that pipeline components and the CLI rely on.
