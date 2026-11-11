# Phase 1 telemetry verification

The explicit smoke program starts a real read-only BlobFuse mount with an
existing private Storage configuration, a private temporary config copy and a
local HTTPS relay forwarding unchanged events to an explicitly chosen test
Application Insights resource. It probes one missing entry, unmounts and checks
attempted/result/stopped ingestion acknowledgements. Phase 1 does not wait for
or require snapshots. The original configuration is not modified, and the
temporary credential-bearing copy is deleted. Private logs/report.json remain;
do not commit them. Ingestion acceptance does not prove query indexing.

```bash
GOEXPERIMENT=systemcrypto ./build.sh
export BLOBFUSE_TELEMETRY_CONNECTION_STRING='<test-connection-string>'
GOEXPERIMENT=systemcrypto go run ./test/telemetry_smoke --binary ./blobfuse2 --config ./config.yaml
```

The relay caps forwarding at 10 requests/80 KiB total and 32 KiB per event.
No failed event is retried. Cleanup targets only the private mount/worker;
forced cleanup fails the run. SIGKILL/host loss can still prevent cleanup.

## Local regression tests

Fixture tests require no Azure traffic:

```bash
GOEXPERIMENT=systemcrypto go test -race -tags=unittest ./test/telemetry_smoke
```

Opt-in real-mount tests use read-only loopback data and local fake HTTPS only:

```bash
GOEXPERIMENT=systemcrypto ./build.sh
BLOBFUSE_TEST_BINARY="$PWD/blobfuse2" GOEXPERIMENT=systemcrypto \
  go test -race -count=1 -v -timeout=5m ./test/telemetry_smoke \
  -run '^(TestRuntimeConfigOptOutMounted|TestStaleMountTelemetryRecovery)$'
```

The opt-out test waits for readiness, atomically edits watched YAML, checks
checkpoint purge and continued file reads, and verifies no shutdown telemetry.
The stale-mount test kills only its private worker, checks ENOTCONN, then remounts
to verify recovery or opted-out purge. Both skip without BLOBFUSE_TEST_BINARY
and are excluded by the unittest tag. The Ubuntu 22 x86 CI job runs them in a
dedicated failure-gating step. Never run alongside host-wide unmount-all tests.
