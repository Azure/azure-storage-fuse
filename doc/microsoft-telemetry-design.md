# Microsoft telemetry implementation

See the [user guide](microsoft-telemetry.md) for configuration, collected data,
event meanings and delivery limits.

## Session and concurrency

Events use Application Insights `EventData` version 2 and BlobFuse
`schema_version` 1, with random event and mount session IDs.

An atomic pointer identifies the active session. One mutex protects metadata,
readiness and result registration through `collecting -> closed`; a background
parent can enter `detached` before closing. Callbacks recheck state and
ownership under the mutex.

The session records one startup result: success at FUSE readiness, or failure
at the first component start error or on close before readiness. Close waits
for an outstanding result outside the mutex; later calls return immediately.

## Initialization and daemon handoff

Startup opt-out creates no collector. Enabled sessions are fully constructed
before publication; watched-config changes do not alter their policy.

The parent passes its monotonic start and opt-out decision to the child, which
generates its own session ID. The launch environment is rebuilt for each
daemonization attempt. The child combines inherited and local startup policy; any disable wins. Missing
parent collection or invalid enabled handoff metadata cannot create an enabled
replacement.

The child owns startup result reporting. Successful parent exit sends
nothing and does not wait for or cancel child delivery. A definitive launch
failure leaves startup failure reporting with the parent.

## Payload validation and delivery

Destination validation requires HTTPS, port 443 and a complete regional
`<region>.in.applicationinsights.azure.com` hostname, with a single DNS label
for the region. Credentials, custom paths, queries and fragments are rejected.
Validation is case-insensitive and performs no network lookup. TLS certificate
verification stays enabled; redirects are disabled. The check does not
authenticate publishers or establish resource ownership.

The encoder allow-lists properties and measurements, rejects
non-finite numbers, and enforces the 32-KiB encoded limit before starting a worker.
Each request contains one result. Responses are not processed or retried.

The delivery worker receives encoded bytes and never accesses session metadata
or the logger. Encoding errors are logged by the session. Closing the session
waits only for an outstanding result, up to the delivery budget, then cancels it
if needed.

## Failure classification

ADLS runs DFS and Blob validation concurrently, retaining DFS error precedence.
Only the selected error sets the startup category. `storageValidationError`
preserves the existing user-facing message and exposes the typed cause through
`Unwrap()` without uploading raw errors or endpoint details.

Telemetry does not reinterpret pipeline errors or change command return behavior.

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
