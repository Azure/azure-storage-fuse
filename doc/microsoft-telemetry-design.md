# Microsoft telemetry implementation

See the [user guide](microsoft-telemetry.md) for configuration, collected data,
event meanings and delivery limits.

## Lifecycle and concurrency

Events use Application Insights `EventData` version 2 and BlobFuse
`schema_version` 1, with random event and mount session IDs.

An atomic pointer identifies the active session. One mutex protects metadata,
readiness and event registration through `collecting -> closing -> closed`;
a background parent can enter `detached` before closing. Callbacks recheck state
and ownership under the mutex.

The session registers its final event before releasing the mutex and closing the
reporter exactly once. Concurrent close callers share session completion, not
potentially late worker cleanup. The reporter needs only a worker wait group and
completion channel, not separate lifecycle guards or a concurrency-admission
counter. At most two lifecycle sends exist per foreground/child process.

## Initialization and daemon handoff

Startup opt-out creates no collector. Enabled sessions are fully constructed
before publication; watched-config changes do not alter their policy.

The parent passes its session ID, monotonic start and opt-out decision to the
child. The launch environment is rebuilt for each daemonization attempt. The
child combines inherited and local startup policy; any disable wins. Missing
parent collection or invalid enabled handoff metadata cannot create an enabled
replacement.

The child owns result and stopped reporting. Successful parent exit sends
nothing and does not wait for or cancel child delivery. A definitive launch
failure leaves startup failure reporting with the parent.

## Payload validation and delivery

Destination validation requires HTTPS, port 443 and a complete regional
`<region>.in.applicationinsights.azure.com` hostname, with a single DNS label
for the region. Credentials, custom paths, queries and fragments are rejected.
Validation is case-insensitive and performs no network lookup. TLS certificate
verification stays enabled; redirects are disabled. The check does not
authenticate publishers or establish resource ownership.

The encoder allow-lists properties and event-specific measurements, rejects
non-finite numbers, and enforces the 32-KiB encoded limit before starting a worker.
Each request contains one event. HTTP 206 is rejected without reading its body;
there is no retry or partial-batch handling.

Workers receive encoded bytes and never access session metadata or the logger.
Delivery failures do not disable later events. Synchronous encoding errors return
to the session for diagnostics. Shutdown waits within its budget, cancels remaining
requests and permits eventual worker cleanup without an unbounded join.

## Failure classification

ADLS runs DFS and Blob validation concurrently, retaining DFS error precedence.
Only the selected error sets the startup category. `storageValidationError`
preserves the existing user-facing message and exposes the typed cause through
`Unwrap()` without uploading raw errors or endpoint details.

A stopped event is clean only after explicit successful pipeline teardown.
Telemetry does not reinterpret pipeline errors or change command return behavior.
Abrupt process death may produce no final event.

## Test isolation

Go test binaries suppress the compiled destination. Ordinary suites and child
launchers also set `BLOBFUSE_DISABLE_TELEMETRY=true`, since separately launched
production binaries do not inherit the Go test guard.

CI establishes opt-out at job/pipeline boundaries. Standalone scripts, benchmarks
and test containers set it explicitly; new launchers must preserve this isolation.

Telemetry tests explicitly enable collection. A test-only CONNECT proxy routes an
approved-format hostname to a local TLS receiver with a temporary trusted
certificate, without external ingestion or a production localhost exception.
The real-mount regression checks background event ownership; see the
[verification instructions](../test/telemetry_smoke/README.md).
