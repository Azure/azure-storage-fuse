# File cache contract

This document lists the behavior of the `file_cache` component that users and other components rely on. Two
engines implement the component:

- **v1**, the original engine, in package `file_cache`. The behavior below is v1 as of commit `c991ed53b`.
- **v2**, the rewrite, in package `filecache`.

Each behavior has an ID. The contract tests, which run against both engines, name in their doc comment the IDs they
pin. The **v2** column says whether v2 keeps the v1 behavior (**Same**), changes it on purpose (**C**_n_, see
[Changes in v2](#3-changes-in-v2)), or waits for a decision (**O**_n_, see [Open decisions](#4-open-decisions)).

Entries were written by reading v1's code. Entries marked _(confirmed)_ were also observed by running v1. A PR that
changes a behavior listed here updates this document in the same PR.

## 1. Interfaces

### 1.1 Component

| ID | Behavior in v1 | Where | v2 |
|---|---|---|---|
| COMP-01 | Registered as `file_cache`, with priority LevelMid: below libfuse, entry_cache, xload and block_cache, above attr_cache and the storage components | `init`, `Priority` | Same |
| COMP-02 | Overrides 29 of the 43 Component methods. CreateDir, OpenDir, CloseDir, SyncDir, CreateLink, ReadLink, UnlinkFile, CopyToFile, CopyFromFile, GetFileBlockOffsets, GetCommittedBlockList, StageData and CommitData go straight to the next component | `file_cache.go` | Same |
| COMP-03 | Registers for config reloads when it is created (RELOAD-01) | `NewFileCacheComponent` | Same |

### 1.2 Settings

All settings are in the `file_cache` section unless they start with `/`, which marks a global setting. Flag defaults
are not applied: a setting that is in neither the config file nor the command line takes the default listed here.

| ID | Setting (flag) | Behavior in v1 | v2 |
|---|---|---|---|
| CONF-01 | `path` (`--tmp-path`) | Cache directory. `~` and environment variables are expanded, and the path made absolute. Required, and must differ from `/mount-path`. Created with mode 0755 if missing | Same |
| CONF-02 | `path` | The mount fails if the directory is not empty, unless `allow-non-empty-temp` is true. The v1 setting `empty-dir-check` (`--empty-dir-check`) is its inverse and wins if set | Same, C17 |
| CONF-03 | `timeout-sec` (`--file-cache-timeout`) | Seconds a file stays cached after its last use; default 120. The v1 setting `file-cache-timeout-in-seconds` wins if set. `/direct-io` forces 0 | Same |
| CONF-04 | `max-size-mb` (`--cache-size-mb`) | Size limit of the cache. If unset or 0: 80% of the free space of the cache disk at mount, or 4192 MB if it cannot be read. Must be greater than 0 | Same, C8 |
| CONF-05 | `high-threshold` (`--high-disk-threshold`), `low-threshold` (`--low-disk-threshold`) | Percentages of `max-size-mb` that start and stop eviction for space (EVICT-03); defaults 80 and 60, although the flag help says 90 and 80 _(confirmed)_ | Same |
| CONF-06 | `max-eviction` (`--max-eviction`) | Maximum number of files evicted in one pass; default 5000 | C2 |
| CONF-07 | `create-empty-file` | CreateFile creates the file in storage right away (CREATE-02) | Same |
| CONF-08 | `offload-io` | If false (the default), libfuse reads and writes the local copy itself (HANDLE-01). If true, reads and writes go through ReadInBuffer and WriteFile | Same |
| CONF-09 | `sync-to-flush` (`--sync-to-flush`), `ignore-sync` (`--ignore-sync`) | Behavior of fsync (SYNC-01 to SYNC-03) | Same |
| CONF-10 | `refresh-sec` | Interval at which an open compares the local copy with storage (OPEN-09); 0 disables it | Same |
| CONF-11 | `hard-limit` (`--hard-limit`) | Operations that would take the cache over its limit fail (USAGE-02) | Same, C7 |
| CONF-12 | `policy` (`--file-cache-policy`), `policy-trace` | `policy` is ignored: eviction is always LRU. `policy-trace` logs the LRU list at each eviction pass | Same |
| CONF-13 | `cleanup-on-start` | Not read by the component: the mount command empties the cache directory before mounting | Same |
| CONF-14 | `background-download`, `cache-poll-timeout-msec`, `upload-modified-only` (hidden flags) | Accepted with a warning; no effect | Same |
| CONF-15 | `/allow-other` | Directories created in the cache, and downloaded files with no mode in storage, get mode 0777 instead of 0755 | Same |
| CONF-16 | `/lazy-write` | Uploads wait for release (FLUSH-03) | Same |
| CONF-17 | — | A config that cannot be parsed, or breaks a rule above, fails the mount with an error naming the setting | Same |
| CONF-18 | — | `blobfuse2 gen-config` writes `path` (the expanded `--tmp-path`) and `timeout-sec` (120, or 0 with `/direct-io`) | Same |

### 1.3 Handles and libfuse

| ID | Behavior in v1 | Where | v2 |
|---|---|---|---|
| HANDLE-01 | OpenFile and CreateFile return a handle whose `Path` is the file name and whose file object and `UnixFD` are the local copy. Unless `offload-io` is set, the handle has `HandleFlagCached`, and libfuse reads and writes the local copy itself, with pread and pwrite | `OpenFile`, `CreateFile` | Same |
| HANDLE-02 | OpenFile sets `Size` to the size of the local copy; CreateFile leaves it 0 | `OpenFile` | Same |
| HANDLE-03 | The handle is dirty after CreateFile (without `create-empty-file`), OpenFile with `O_TRUNC`, WriteFile and TruncateFile with a handle. libfuse marks it dirty for its own writes before flush, fsync and release, and calls FlushFile only for a dirty handle | `handle_map.go`, libfuse | Same |
| HANDLE-04 | During an upload, the dirty state moves to `HandleFlagFlushing`, and `Dirty()` reports both. A write during the upload keeps the handle dirty, so the next flush or release uploads again (issue #2348) | `FlushFile` | Same |
| HANDLE-05 | SyncFile sets `HandleFlagFSynced` when it calls the next component (SYNC-02) | `SyncFile` | Same |
| HANDLE-06 | A handle keeps the name it was opened with, also after the file is renamed (REN-04) | — | C14 |
| HANDLE-07 | libfuse frees the handle even if ReleaseFile fails | libfuse, PR #2427 | Same |

### 1.4 Cache directory

| ID | Behavior in v1 | Where | v2 |
|---|---|---|---|
| LAYOUT-01 | The local copy of `a/b` is `<path>/a/b`. Parent directories are created as needed, with mode 0755 (0777 with `/allow-other`) | `OpenFile`, `CreateFile` | Same |
| LAYOUT-02 | Downloads write to that path directly. The health monitor watches the directory for create, write, remove, rename and chmod events | `OpenFile`, health monitor | Same |
| LAYOUT-03 | After a download, the local copy has the storage mode (or 0755, 0777 with `/allow-other`, if storage has none) and the storage access and modification times, so its change time is the download time | `OpenFile` | Same |
| LAYOUT-04 | Stop deletes everything in the cache directory | `Stop` | Same |
| LAYOUT-05 | Eviction deletes files and leaves their directories, even when empty | `deleteItem` | Same |

### 1.5 Statistics

| ID | Behavior in v1 | Where | v2 |
|---|---|---|---|
| STATS-01 | The collector is named `file_cache`. `Files Downloaded` and `Files served from cache` count the opens that did and did not download the file | `OpenFile` | Same |
| STATS-02 | `Cache Usage` (`"<MB> MB"`) and `Usage Percent` (`"<percent>%"`) are updated once a minute, when `du` works | `getUsagePercentage` | Same |

## 2. Behavior

### Names

| ID | Behavior in v1 | Where | v2 |
|---|---|---|---|
| NAME-01 | Methods that take a name fail with EINVAL for an absolute name, a name containing `\`, or a name that leaves the root through `..`. The empty name is the root. IsDirEmpty returns false instead, and GetAttr returns an empty attribute with EINVAL | `validateObjectName` | Same |
| NAME-02 | Methods that take a handle use the name stored in the handle | — | Same, C14 |

### Start and stop

| ID | Behavior in v1 | Where | v2 |
|---|---|---|---|
| LIFE-01 | Start starts eviction and the statistics collector. It runs `du` once, and turns off eviction for space (EVICT-03) if `du` fails | `Start`, `StartPolicy` | C1 |
| LIFE-02 | Stop waits for ReleaseFile calls in progress when `/lazy-write` is set, stops eviction and empties the cache directory | `Stop` | Same |
| LIFE-03 | Stop does not wait for the cleanup started by DeleteDir and RenameDir (DIR-04) | `DeleteDir`, `RenameDir` | C4 |

### Open

| ID | Behavior in v1 | Where | v2 |
|---|---|---|---|
| OPEN-01 | Opening a file with no local copy downloads it: GetAttr, then CopyToFile of the whole file into the local copy | `OpenFile` | Same |
| OPEN-02 | A local copy that was downloaded or changed less than `timeout-sec` ago is used without asking storage. With `timeout-sec` 0, it is downloaded on every open | `isDownloadRequired` | Same |
| OPEN-03 | A local copy is never downloaded again while the file has open handles | `isDownloadRequired` | Same |
| OPEN-04 | Opening with `O_TRUNC` creates an empty local copy without downloading, and the handle is dirty | `OpenFile` | Same |
| OPEN-05 | If the download fails, the partial local copy is deleted and the error returned | `OpenFile` | Same |
| OPEN-06 | If GetAttr fails with EACCES, the open fails with EACCES | `OpenFile` | Same |
| OPEN-07 | If a download is needed and GetAttr fails with another error, including ENOENT, the local copy is replaced by an empty file, which is opened. Written to, it is uploaded over the file in storage | `OpenFile` | C9 |
| OPEN-08 | The local copy is opened with the caller's flags and mode, and errors are returned as they are | `OpenFile` | Same |
| OPEN-09 | With `refresh-sec`, an open more than `refresh-sec` after the last download compares the storage modification time and size with the local copy, and downloads the file if storage is newer or the size differs. v1 does so even when the file has open handles, and asks storage on every open once the interval has passed | `isDownloadRequired` | Same, C10 |

### Create

| ID | Behavior in v1 | Where | v2 |
|---|---|---|---|
| CREATE-01 | CreateFile creates or truncates the local copy with the given mode, and returns a dirty handle. The file reaches storage when it is flushed or released | `CreateFile` | Same |
| CREATE-02 | With `create-empty-file`, CreateFile first creates the file in storage, and fails if that fails. The handle is not dirty | `CreateFile` | Same |
| CREATE-03 | If the mode is not 0755, the mode of the local copy is set in storage after the first upload (FLUSH-06) | `CreateFile` | Same |

### Read and write

| ID | Behavior in v1 | Where | v2 |
|---|---|---|---|
| IO-01 | ReadInBuffer reads the local copy at the given offset. WriteFile writes it and marks the handle dirty. Both are used only with `offload-io` | `ReadInBuffer`, `WriteFile` | Same |
| IO-02 | A handle without a local file fails with EBADF in ReadInBuffer, WriteFile, ReadFile, TruncateFile and ReleaseFile, and in FlushFile if the handle is dirty | — | Same |
| IO-03 | ReadFile returns the local copy from the handle's current offset, so a second call fails with EIO. No production code calls it | `ReadFile` | C12 |

### Flush

| ID | Behavior in v1 | Where | v2 |
|---|---|---|---|
| FLUSH-01 | FlushFile of a handle that is not dirty does nothing | `FlushFile` | Same |
| FLUSH-02 | FlushFile of a dirty handle uploads the whole local copy and clears the dirty state. If the upload fails, it returns the error and the handle stays dirty | `FlushFile` | Same |
| FLUSH-03 | With `/lazy-write`, FlushFile uploads only when the file is being closed: from ReleaseFile, or from SyncFile with `sync-to-flush` | `FlushFile` | Same |
| FLUSH-04 | Uploads of the same file never overlap. A flush that waited for another upload of the same handle, and finds the handle clean, does nothing | `FlushFile` | Same |
| FLUSH-05 | If the local copy has no read permission, read permission is added for the upload and removed afterwards | `FlushFile` | Same |
| FLUSH-06 | After an upload, a mode change waiting for it (CREATE-03, MODE-02) is applied in storage. If that fails, it is logged and forgotten | `FlushFile` | C13 |

### Release

| ID | Behavior in v1 | Where | v2 |
|---|---|---|---|
| REL-01 | ReleaseFile uploads a dirty handle (FLUSH-02) and closes it. If the upload fails, it returns the error, and the local copy stays until it is evicted | `ReleaseFile` | Same, O5 |
| REL-02 | When the last handle is released with `timeout-sec` 0, the local copy is deleted. Otherwise it stays until it is evicted | `ReleaseFile`, `CacheInvalidate` | Same |
| REL-03 | After an fsync that called the next component (SYNC-02), releasing that handle deletes the local copy, unless other handles are open | `ReleaseFile` | Same |
| REL-04 | Releasing a dirty handle of a file that was deleted or renamed while open fails with ENOENT _(confirmed)_ | `ReleaseFile` | C14, C15 |

### Sync

| ID | Behavior in v1 | Where | v2 |
|---|---|---|---|
| SYNC-01 | With `sync-to-flush`, SyncFile uploads like a release (FLUSH-03) and returns the upload error | `SyncFile` | Same |
| SYNC-02 | Otherwise, unless `ignore-sync` is set, SyncFile calls the next component's SyncFile and marks the handle (REL-03) | `SyncFile` | Same |
| SYNC-03 | With `ignore-sync`, SyncFile does nothing | `SyncFile` | Same |

### Truncate

| ID | Behavior in v1 | Where | v2 |
|---|---|---|---|
| TRUNC-01 | TruncateFile with a handle truncates the local copy and marks the handle dirty. Storage changes at the next upload | `TruncateFile` | Same |
| TRUNC-02 | TruncateFile without a handle truncates in storage, then the local copy if there is one. A file only in the local cache is truncated locally; a file in neither fails with ENOENT | `TruncateFile` | Same, O3 |

### Attributes

| ID | Behavior in v1 | Where | v2 |
|---|---|---|---|
| ATTR-01 | GetAttr returns the storage attributes. If there is a local copy, its size and modification time replace the storage ones, unless an operation on the file is in progress | `GetAttr` | Same, C3 |
| ATTR-02 | A file only in the local cache gets the attributes of its local copy | `GetAttr`, `newObjAttr` | Same |
| ATTR-03 | A name in neither fails with ENOENT. Other storage errors are returned | `GetAttr` | Same |
| ATTR-04 | Directories always get the storage attributes | `GetAttr` | Same |
| ATTR-05 | v1 changes the attributes returned by the next component in place, although attr_cache keeps them | `GetAttr`, `ReadDir` | C11 |

### Listing

| ID | Behavior in v1 | Where | v2 |
|---|---|---|---|
| LIST-01 | StreamDir returns the storage page. On the last page, it adds the files that are only in the local cache, which it finds by calling GetAttr on storage for each local file. Files in both keep the storage size | `StreamDir` | Same, O4 |
| LIST-02 | If storage fails on the last page, StreamDir still adds the local files and returns the error | `StreamDir` | Same |
| LIST-03 | ReadDir lists storage and the local cache like StreamDir, but gives files in both the local size and time, and ignores storage errors. No production code calls it | `ReadDir` | Same |

### Directories

| ID | Behavior in v1 | Where | v2 |
|---|---|---|---|
| DIR-01 | IsDirEmpty is false if the directory is not empty in storage. Otherwise it is true only if the local directory has no file at any depth, and then the empty local directories are deleted | `IsDirEmpty`, `deleteEmptyDirs` | Same |
| DIR-02 | DeleteDir deletes the directory in storage, then the local directory and every cached file in it, also when storage failed. It returns the storage error | `DeleteDir` | Same |
| DIR-03 | RenameDir renames the directory in storage, then deletes the source directory from the local cache. libfuse makes sure the destination is empty | `RenameDir` | Same |
| DIR-04 | The local cleanup of DIR-02 and DIR-03 runs in the background, after the call returns | `invalidateDirectory` | C4 |

### Delete

| ID | Behavior in v1 | Where | v2 |
|---|---|---|---|
| DEL-01 | DeleteFile deletes the file in storage, then its local copy | `DeleteFile` | Same |
| DEL-02 | A file only in the local cache (created and not uploaded yet) fails with EIO | `validateStorageError`, `TestDeleteFileCase2` | O1 |
| DEL-03 | A file with open handles is deleted, and releasing a dirty handle then fails (REL-04) _(confirmed)_ | `DeleteFile` | C15 |

### Rename

| ID | Behavior in v1 | Where | v2 |
|---|---|---|---|
| REN-01 | RenameFile renames the file in storage, then the local copy, replacing a cached destination. If the local rename fails, the cached destination is deleted | `RenameFile` | Same |
| REN-02 | A source only in the local cache fails with EIO | `validateStorageError`, `TestRenameFileCase2` | O1 |
| REN-03 | With `timeout-sec` 0, the local copy of the destination is deleted when not in use. Otherwise it stays until it is evicted | `RenameFile` | Same |
| REN-04 | Handles of the source keep the old name. Their next upload fails, and what was written since the last upload is lost _(confirmed)_ | `RenameFile` | C14 |

### Mode and owner

| ID | Behavior in v1 | Where | v2 |
|---|---|---|---|
| MODE-01 | Chmod changes the mode in storage, then of the local copy | `Chmod` | Same |
| MODE-02 | For a file only in the local cache, Chmod changes the local copy and succeeds; the mode is set in storage after the next upload (FLUSH-06) | `Chmod` | Same |
| MODE-03 | v1 also handles a genuine EIO from storage as in MODE-02, and returns success | `Chmod` | C13 |
| MODE-04 | Chown changes the owner in storage, then of the local copy. A file only in the local cache fails with EIO | `Chown` | O2 |

### Eviction

| ID | Behavior in v1 | Where | v2 |
|---|---|---|---|
| EVICT-01 | With `timeout-sec` above 0, the local copy of a file is deleted after the file is unused for between one and two timeouts | `lruPolicy` | Same |
| EVICT-02 | A file with open handles is never evicted | `deleteItem` | Same |
| EVICT-03 | Once a minute, if the cache uses more than `high-threshold` percent of `max-size-mb`, the least recently used files are evicted until usage drops below `low-threshold` percent, or after 3 rounds. Usage is measured with `du` | `clearCache` | C1, C5 |
| EVICT-04 | These count as a use of the file: open, create, flush, release, truncate, chmod, chown, and rename for the destination. ReadInBuffer and WriteFile count once every 100 calls. Reads and writes done by libfuse itself (HANDLE-01) do not count | `CacheValid` callers | Same |

### Disk usage

| ID | Behavior in v1 | Where | v2 |
|---|---|---|---|
| USAGE-01 | StatFs reports `max-size-mb` as the size of the file system, and the space left in the cache, by `du`, as its free and available space. The other fields come from `/` | `StatFs` | C1, C6 |
| USAGE-02 | With `hard-limit`, a download at open, a WriteFile, and a TruncateFile fail with ENOSPC if the cache would exceed `high-threshold` percent of `max-size-mb`. v1 adds megabytes to bytes, so it almost never fails, and a truncate fails for any size above the limit, even when the file shrinks or is not cached | `OpenFile`, `WriteFile`, `TruncateFile` | C7 |

### Config reload

| ID | Behavior in v1 | Where | v2 |
|---|---|---|---|
| RELOAD-01 | When the config file changes while mounted, v1 applies `create-empty-file`, `timeout-sec` (but not to eviction), `policy-trace`, `offload-io`, `max-size-mb`, `sync-to-flush`, `ignore-sync`, the thresholds and `max-eviction`. Other settings need a remount | `OnConfigChange` | Same |
| RELOAD-02 | v1 applies a config it cannot parse as all zeros, sets the timeout to 0 when `timeout-sec` is not set, and keeps the hard limit of the old `max-size-mb` | `OnConfigChange` | C16 |

## 3. Changes in v2

| ID | Change | Reason |
|---|---|---|
| C1 | No `du`. The engine counts the disk space of the files it caches. Usage reports may differ a little from `du`, and files already in the cache directory at mount are not counted (C17) | `du` can block for minutes on large caches or slow disks |
| C2 | `max-eviction` is accepted, and ignored with a warning | Eviction works in bytes, not in numbers of files |
| C3 | While a file is uploaded, GetAttr returns the size and time of the local copy. Only a download in progress hides the local copy | v1 hides it during any operation on the file, which is not needed once the copy is complete |
| C4 | DeleteDir and RenameDir clean up the local cache before they return | No work left running after the call, or after Stop |
| C5 | Eviction for space starts as soon as usage crosses `high-threshold`, instead of at the next one-minute check | Usage is known at every change |
| C6 | StatFs takes the fields other than the size from the cache disk, not from `/` | The size fields describe the cache, so the others should too |
| C7 | The hard limit counts the space already used by the cache, and only growth counts for truncate. Writes done by libfuse itself (HANDLE-01) are not limited, as in v1 | Fixes USAGE-02. Users with `hard-limit` may see ENOSPC that v1 never returned |
| C8 | If the free space of the cache disk cannot be read, `max-size-mb` defaults to 4096, not 4192 | 4192 was a typo |
| C9 | If a download is needed and GetAttr fails, the open fails with that error. If the error is ENOENT and a local copy exists, the local copy is opened | Fixes OPEN-07, which could upload an empty file over the file in storage |
| C10 | With `refresh-sec`, a file with open handles is never downloaded again, and after a check that finds the copy current, the next check waits a full interval | Fixes OPEN-09, which could lose writes of open handles |
| C11 | Attributes from the next component are copied before the local size and time are applied | Fixes ATTR-05 |
| C12 | ReadFile returns the whole local copy, whatever the handle's offset | Fixes IO-03 |
| C13 | Chmod returns storage errors other than "does not exist". A mode change waiting for an upload is retried after the next upload if it fails | Fixes MODE-03 and FLUSH-06 |
| C14 | A handle follows its file when the file is renamed, and its upload goes to the new name | Fixes REN-04, which loses data |
| C15 | Releasing a dirty handle of a deleted file succeeds and uploads nothing | POSIX: closing an unlinked file succeeds and does not bring it back |
| C16 | A config reload follows the same rules as the mount: unparsable configs are ignored, unset settings take their defaults, and the hard limit follows `max-size-mb` | Fixes RELOAD-02 |
| C17 | Files found in the cache directory at mount are not counted, and are never served as cached copies: opening such a name downloads the file over it. v1 serves them without asking storage if they changed less than `timeout-sec` ago | Counting them would need a scan at mount, and serving them can return stale data |

## 4. Open decisions

| ID | Question | Recommendation |
|---|---|---|
| O1 | Deleting or renaming a file that is not uploaded yet fails with EIO in v1 (DEL-02, REN-02). Keep it? | Change: delete drops the local copy without uploading it, and rename moves it, to be uploaded under the new name. Tools that write a temporary file and rename it before closing it fail on v1 |
| O2 | Chown of a file that is not uploaded yet fails with EIO in v1 (MODE-04). Keep it? | Keep it. Owners are rarely changed through the mount |
| O3 | With `create-empty-file`, v1 lets DeleteFile, RenameFile, TruncateFile, Chmod and Chown succeed for a name that is in neither storage nor the cache. Keep it? | Change: fail with ENOENT |
| O4 | StreamDir shows the storage size of a file that is being written (LIST-01), while GetAttr shows the local size (ATTR-01), so `ls -l` and `stat` disagree. Keep it? | Change: StreamDir shows the local size too |
| O5 | A file whose upload failed at release stays in the cache until it is evicted, and its changes are then lost (REL-01). Keep it? | Keep it for January. Retrying failed uploads is a feature of its own |
