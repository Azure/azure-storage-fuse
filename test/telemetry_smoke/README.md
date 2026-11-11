# VM-to-Application-Insights smoke test

This explicitly invoked workflow tests the actual local BlobFuse binary against
an existing storage account and a test Application Insights destination.
It is not started by `go test` or CI.

From the repository root:

```bash
GOEXPERIMENT=systemcrypto ./build.sh
export BLOBFUSE_TELEMETRY_CONNECTION_STRING='<test-connection-string>'
GOTOOLCHAIN=local GOEXPERIMENT=systemcrypto CGO_ENABLED=1 \
  go run ./test/telemetry_smoke \
  --binary ./blobfuse2 \
  --config ./config.yaml
```

The original configuration and storage credentials are never modified or printed.
The test destination is read only from
`BLOBFUSE_TELEMETRY_CONNECTION_STRING`. The workflow refuses to run without this
explicit override and never falls back to the product's compiled production
destination. There is no destination YAML setting or destination CLI flag.
The workflow:

1. Creates a private test directory and a temporary credential-bearing config.
2. Uses the existing storage configuration with a small file-cache pipeline,
   read-only mounting, no preload, and isolated telemetry state.
3. Starts a temporary loopback HTTPS verification relay. Only the child process
   trusts its certificate; normal Azure certificate roots remain available.
   The relay forwards the real BlobFuse event envelopes unchanged to the
   configured Application Insights endpoint and captures ingestion acknowledgements.
   This is a test fixture, not a production gateway.
4. Mounts with the actual binary, probes a random missing entry, and waits
   70 seconds for a one-minute snapshot (including jitter). No blob contents
   are downloaded, and no blobs are created, modified, or deleted.
5. Unmounts, waits for shutdown delivery, and verifies acknowledged attempted,
   result, snapshot, and stopped events, including successful mount and explicitly
   completed clean shutdown.
6. Deletes the temporary credential-bearing config. Private logs and a sanitized
   `report.json` remain for review.

Forwarding is limited to **10 requests and 80 KiB of JSON payload**, with an
32-KiB per-request limit. The total 80-KiB forwarding budget remains independent
of the event-size limit. HTTPS overhead is additional. Normal runs send about
five events; snapshot splitting may change the count within the cap.
Each event has one send opportunity; ingestion failures do not trigger retries.
Cleanup retries unmount and uses lazy unmount/termination only for its own
private test mount/worker if necessary; any forced cleanup fails the test.
SIGINT/SIGTERM request cleanup; SIGKILL or host loss cannot guarantee cleanup.

Artifacts default to a fresh directory under
`$XDG_CACHE_HOME/blobfuse2-telemetry-smoke` (or the platform user-cache default).
`--output` selects another parent directory. Do not commit generated configs or
logs: local diagnostic logs may contain storage identifiers.

Acceptance responses establish that Azure ingestion acknowledged the events.
They do not independently verify indexing/query visibility. In the test
Application Insights resource's Logs view, use the session ID from `report.json`:

```kusto
customEvents
| where timestamp > ago(2h)
| where tostring(customDimensions.mount_session_id) == "<session-id>"
| project timestamp, name, customDimensions, customMeasurements
| order by timestamp asc
```

Fixture tests use only a local fake HTTPS server and send nothing to Azure:

```bash
GOEXPERIMENT=systemcrypto go test -race ./test/telemetry_smoke
```

For fixture-only unit runs, use `-tags=unittest` to exclude the real-mount test
files even when `BLOBFUSE_TEST_BINARY` is set:

```bash
GOEXPERIMENT=systemcrypto go test -race -tags=unittest ./test/telemetry_smoke
```

The Ubuntu 22 x86 CI job runs both real-mount regressions in a separate,
failure-gating step using its freshly built binary. These tests override the
child's telemetry destination with a local fake HTTPS receiver; the manual
Azure-forwarding smoke workflow is not invoked by CI. Do not run these mount
tests concurrently with host-wide mount/unmount tests.

## Live configuration opt-out integration test

This opt-in test starts the actual binary with a private read-only loopback FUSE
mount and a local fake HTTPS receiver. It does not access Azure Storage or
Application Insights, even though the binary contains a compiled test destination.

```bash
GOEXPERIMENT=systemcrypto ./build.sh
BLOBFUSE_TEST_BINARY="$PWD/blobfuse2" GOEXPERIMENT=systemcrypto CGO_ENABLED=1 \
  go test -race -count=1 -v -timeout=4m ./test/telemetry_smoke \
  -run '^TestRuntimeConfigOptOutMounted$'
```

It verifies readiness and a periodic snapshot, then atomically replaces the
watched YAML with `microsoft-telemetry.disabled: true`. It checks that running
session state is purged, file reads still work, no further telemetry
arrives over a complete maximum-jitter reporting interval, and unmount/shutdown
does not recreate state or emit a stopped event. The fake receiver returns 503
to verify that failed one-shot sends do not stop later snapshot reporting.
The running-session checkpoint remains available for the purge assertion.
Only private test configuration is edited; the caller's configuration is untouched.
Cleanup unmounts the test filesystem and waits for the worker to exit.

Without `BLOBFUSE_TEST_BINARY`, this integration test is skipped. It requires a
Linux VM with functional FUSE and the freshly built loopback-capable binary,
takes approximately two minutes, and covers foreground execution/local file
watching, not CSI configuration projection or every daemon/signal path.

## Stale-mount recovery integration test

This explicitly opted-in test kills only its own read-only local loopback worker
and verifies that a FUSE mount remains with `ENOTCONN` during path resolution.
It then launches BlobFuse again at the same path, relying on existing stale-mount
cleanup. The enabled case checks recovery of the previous session, durable
readiness for the new session, usable file reads, and no duplicate startup
events. The opted-out case checks that old state is purged without transmitting
new events. Each case unmounts and joins its worker during cleanup.

```bash
GOEXPERIMENT=systemcrypto ./build.sh
BLOBFUSE_TEST_BINARY="$PWD/blobfuse2" GOEXPERIMENT=systemcrypto CGO_ENABLED=1 \
  go test -race -count=1 -v -timeout=3m ./test/telemetry_smoke \
  -run '^TestStaleMountTelemetryRecovery$'
```

Only a local fake HTTPS receiver is used; there are no Azure Storage or
Application Insights calls. The test is skipped unless the binary path is
provided, and the affected case is skipped if the kernel does not expose the
specific disconnected-path condition. SIGKILL or host loss of the test runner
itself can still prevent cleanup.
