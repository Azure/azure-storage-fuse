# Local telemetry mount regression

This opt-in test uses a read-only loopback mount and a local HTTPS receiver.
It requires FUSE and a freshly built BlobFuse binary, but no Azure credentials
or external ingestion service.
An approved-format ingestion hostname is routed through a local CONNECT proxy
to the test receiver. Its temporary certificate is trusted only by the test
client/child environment; production hostname validation is not bypassed.

Unit coverage for delivery, payload validation and lifecycle concurrency lives
in `internal/telemetry`. Run the real-mount regression with:

```bash
GOEXPERIMENT=systemcrypto ./build.sh
BLOBFUSE_TEST_BINARY="$PWD/blobfuse2" GOEXPERIMENT=systemcrypto \
  go test -race -count=1 -v -timeout=5m ./test/telemetry_smoke \
  -run '^TestBackgroundAttemptOwnedByDaemon$'
```

The background ownership test holds the attempted-event
HTTPS response open, verifies the launching parent exits without waiting or
cancelling it, then checks that the daemon sends exactly one attempted, result
and stopped event under the same session identity.
It skips without `BLOBFUSE_TEST_BINARY`, is excluded by the `unittest` tag,
and runs in a dedicated Ubuntu 22 x86 CI step.
Never run alongside host-wide unmount-all tests.
Recovery, state-retention and stale-mount recovery scenarios are outside Phase 1.
