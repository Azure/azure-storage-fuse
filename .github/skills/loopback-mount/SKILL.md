---
name: loopback-mount
description: 'Mount blobfuse2 with FUSE over a local directory (loopbackfs) instead of Azure Storage, to reproduce or verify filesystem behavior end to end without credentials. Use when checking what applications see through the mount (errno values, rename, fsync, close, permissions, caching) or debugging a hang, and before claiming a FUSE-visible fix works.'
---

# Mount over a local directory

`loopbackfs` is a component that stores files in a local directory, so `libfuse → file_cache → loopbackfs` exercises the real kernel, libfuse and file cache with no storage account. It needs `/dev/fuse` and `fusermount3` (`fusermount` with fuse2).

## Automated test

```bash
make test-loopback
```

This builds `./blobfuse2` and runs `test/loopback_e2e`, a concurrent close, fsync and rename workload that fails when the mount stops making progress. For a shorter run after `make build`, use `go test -count=1 ./test/loopback_e2e -args -blobfuse2=$PWD/blobfuse2 -duration=10s`. Add a new end-to-end check to that package when a unit test can't show the behavior.

## Manual mount

```bash
make build
W=$(mktemp -d) && mkdir -p "$W/mnt" "$W/cache" "$W/storage"
cat > "$W/config.yaml" <<EOF
logging:
  type: base
  level: log_debug
  file-path: $W/blobfuse2.log
components:
  - libfuse
  - file_cache
  - loopbackfs
libfuse:
  attribute-expiration-sec: 0
  entry-expiration-sec: 0
  negative-entry-expiration-sec: 0
file_cache:
  path: $W/cache
  timeout-sec: 0
loopbackfs:
  path: $W/storage
EOF
./blobfuse2 mount "$W/mnt" --config-file="$W/config.yaml" --disable-version-check=true
for i in $(seq 50); do grep -q " $W/mnt " /proc/self/mountinfo && break; sleep 0.2; done
```

The mount runs in the background. Then:

1. Run the operation under test against `$W/mnt` and check the result in `$W/storage`, which plays the role of the container. For example, `echo hello > "$W/mnt/a.txt" && cat "$W/storage/a.txt"`.
2. Read `$W/blobfuse2.log` for the calls each component received; `grep -E 'LOG_ERR|LOG_CRIT' "$W/blobfuse2.log"` shows failures.
3. Unmount with `fusermount3 -u "$W/mnt"`. If it reports the mount is busy or hung, run `fusermount3 -u -z "$W/mnt"`, then stop the process with `kill <PID>` (find it with `pgrep -f "blobfuse2 mount $W/mnt"`).
4. Remove the directory with `rm -rf "$W"`.

To see a crash or panic as it happens, add `--foreground=true` to the mount command and run it in a separate terminal.

## Variations

- Add `attr_cache` after `file_cache` to include attribute caching, and set the option you are testing in its section of the config.
- Set `libfuse.direct-io: true` to bypass the kernel page cache; `test/loopback_e2e` uses it to reproduce the file-cache hang fixed in PR #2383.
- To test streaming mode, replace the `file_cache` entry and section with `block_cache` and a section such as `block_cache: {block-size-mb: 1, mem-size-mb: 64, path: $W/cache, disk-size-mb: 128}`. Small blocks make a file of a few MB span several blocks.
- Never mount a real container this way, and never point `loopbackfs.path` at a directory whose contents you need.
