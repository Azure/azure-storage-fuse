# Local telemetry mount regression

This opt-in test requires FUSE and a freshly built BlobFuse binary. It uses a
read-only loopback mount and a local TLS receiver through a test-only CONNECT
proxy. No Azure credentials, external ingestion or production validation bypass
are needed.

Unit coverage for delivery, payload validation and lifecycle concurrency lives
in `internal/telemetry`. Run the real-mount regression with:

```bash
GOEXPERIMENT=systemcrypto ./build.sh
BLOBFUSE_TEST_BINARY="$PWD/blobfuse2" GOEXPERIMENT=systemcrypto \
  go test -race -count=1 -v -timeout=5m ./test/telemetry_smoke \
  -run '^TestBackgroundStartupOwnedByDaemon$'
```

The test blocks the result response, checks that the parent exits without
waiting or canceling child delivery, then verifies one result and one stopped
event under the same session ID.
It skips without `BLOBFUSE_TEST_BINARY`, is excluded by the `unittest` tag,
and runs in a dedicated Ubuntu 22 x86 CI step.
Never run alongside host-wide unmount-all tests.
