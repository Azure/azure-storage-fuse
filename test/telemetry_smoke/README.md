# Local telemetry mount regressions

These opt-in tests use read-only loopback data and a local HTTPS receiver.
They require FUSE and a freshly built BlobFuse binary, but no Azure credentials
or external ingestion service.

Unit coverage for delivery, payload validation and lifecycle concurrency lives
in `internal/telemetry`. Run the real-mount regressions with:

```bash
GOEXPERIMENT=systemcrypto ./build.sh
BLOBFUSE_TEST_BINARY="$PWD/blobfuse2" GOEXPERIMENT=systemcrypto \
  go test -race -count=1 -v -timeout=5m ./test/telemetry_smoke \
  -run '^Test(RuntimeConfigOptOutMounted|BackgroundAttemptOwnedByDaemon)$'
```

The opt-out test waits for readiness, atomically edits watched YAML to disable telemetry,
waits for the watcher notification, checks continued file reads and verifies
that shutdown sends no stopped event. It also checks that no telemetry recovery
directory was created. The background ownership test holds the attempted-event
HTTPS response open, verifies the launching parent exits without waiting or
cancelling it, then checks that the daemon sends exactly one attempted, result
and stopped event under the same session identity.
Both skip without `BLOBFUSE_TEST_BINARY`, are excluded by the `unittest` tag,
and run in a dedicated Ubuntu 22 x86 CI step.
Never run alongside host-wide unmount-all tests.
Recovery, state-retention and stale-mount recovery scenarios are outside Phase 1.
