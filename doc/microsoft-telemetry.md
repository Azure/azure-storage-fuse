# Microsoft lifecycle and error telemetry

BlobFuse can collect structured mount lifecycle events, approved effective
configuration, and aggregate error/retry counters for a Microsoft-owned
Application Insights resource. This is separate from local diagnostic logging
and health-monitor output. It does not collect file contents, paths, account or
container names, credentials, raw configuration, or raw error text.

## Enablement and opt-out

Telemetry policy is enabled by default. The shared Microsoft destination is the
`telemetryConnectionString` constant in `cmd/telemetry.go`, following AzCopy's
compiled-default model. This development branch currently embeds the personal
**TEST resource**, so ordinary mounts can report there without an environment
override unless opted out. **Remove/replace that test connection string with the
approved BlobFuse production destination before merging to main or releasing.**
It is not a production resource; do not reuse AzCopy's resource either.

```yaml
microsoft-telemetry:
  disabled: false
  snapshot-interval-sec: 300
  # Optional. Must be a dedicated private directory ending in microsoft-telemetry.
  # state-directory: /persistent/blobfuse/microsoft-telemetry
```

The only runtime destination override is
`BLOBFUSE_TELEMETRY_CONNECTION_STRING`. A nonempty trimmed value overrides the
compiled default. Destination selection through YAML or a CLI flag is not
supported. `setup/baseConfig.yaml` documents opt-out, cadence and state settings,
but does not contain an ingestion destination.

Go test binaries ignore the compiled default and report only to an explicitly
supplied destination. Tests launching the ordinary production executable must
explicitly opt out or inject an approved test destination.

For development testing, explicitly export the test connection string:

```bash
export BLOBFUSE_TELEMETRY_CONNECTION_STRING='<test-connection-string>'
GOEXPERIMENT=systemcrypto go run ./test/telemetry_smoke \
  --config ./config.yaml
```

The smoke workflow requires this explicit override even when a compiled
production default exists, to avoid accidentally testing against production.
The instrumentation key is a public routing identifier, not sender
authentication. Clear retained test records before switching resources, and
unset the environment override when returning to the compiled destination.

Any of these disables Microsoft telemetry, regardless of an explicit enable
elsewhere:

```yaml
microsoft-telemetry:
  disabled: true
```

```bash
export BLOBFUSE_DISABLE_TELEMETRY=true
blobfuse2 mount /mnt/blobdata --config-file=config.yaml --disable-telemetry
```

The configuration switch, environment variable, and flag use **disable-wins**
semantics, not normal overriding precedence. Opt-out deletes recognized retained
telemetry records only in this logical mount's store; failures to access/delete that store
are diagnosed locally. Files outside that dedicated store are not deleted.
Changing the watched configuration to disabled cancels reporting and purges the
active store. Re-enablement and other telemetry-setting changes require a new
mount. Diagnostic logging and storage authentication are unchanged.

Live YAML disabling is covered by the opt-in
`TestRuntimeConfigOptOutMounted` integration test, using a real foreground
loopback mount and local HTTPS receiver. It edits the watched file, verifies
purged state and continued file access, waits a full reporting interval for
absence of further telemetry, and unmounts. See
[test instructions](../test/telemetry_smoke/README.md#live-configuration-opt-out-integration-test).
This does not establish that every container/CSI file projection delivers
configuration changes. Daemon-handoff opt-out has separate local regression
tests for configuration callbacks, disabled/detached owners, and concurrent
failure delivery.

## Records and cadence

| Record | Purpose |
|---|---|
| `blobfuse.mount.attempted` | One actual instrumented mount attempt |
| `blobfuse.mount.result` | Confirmed FUSE readiness or definitive startup failure |
| `blobfuse.mount.snapshot` | Session-cumulative HTTP/SDK/transport/FUSE counters and uptime |
| `blobfuse.mount.stopped` | Observed exit/teardown outcome and final known uptime |
| `blobfuse.mount.recovery` | Later discovery of an unfinished previous session |

Collection starts after configuration can be read and telemetry policy resolved.
Failures before that boundary (including invalid configuration syntax or a
loader failure) cannot be reported by this implementation.

Lifecycle records are dispatched asynchronously. After readiness, snapshots are taken every
300 seconds with up to 10% jitter. The configurable interval must be between
60 and 3600 seconds. A final snapshot is dispatched when mounted execution returns.
Each event gets one send opportunity, regardless of acceptance. Delivery failure
does not disable collection or future sends. There is no in-memory waiting queue,
retry timer, backoff or retry spool. Periodic/final snapshots are memory-only.

Background mounts share one session identity across the launching parent
and daemon. Startup counters are transferred through protected state where
available. Startup elapsed time uses the host monotonic clock across re-exec;
uptime starts at confirmed FUSE initialization. Readiness is not a guarantee that
every future storage operation will succeed. A startup wait timeout is not
classified as a definitive failure.

If telemetry initially cannot resolve a disconnected/stale mount path or open
its protected state, it remains memory-only until mount-path validation. After
the existing stale-mount cleanup succeeds, BlobFuse retries state attachment,
recovers eligible earlier sessions and checkpoints the current session.
Already dispatched events are not persisted or sent again. It does not
create a second mount attempt or reporter. This scoped retry also completes
deferred opt-out cleanup without re-enabling telemetry. If state remains
unavailable, the mount continues with diagnosed degraded telemetry.

Parent detachment stops counter/snapshot collection but retains the existing
reporter until the launch outcome. A failed launch uses that reporter instead of
starting a replacement session. Explicit opt-out is sticky, remains reachable
through configuration updates while detached, and cancels in-flight delivery
before waiting for lifecycle cleanup. Failure reporting cannot restart it.

Clean shutdown requires an explicit completion marker after all pipeline
components stop successfully. A nil error or execution of a deferred function
is not sufficient. Incomplete teardown without a returned error is recorded as
`unclean_unknown`; a controlled returned error is `runtime_error`. A panic is not
recovered or swallowed by telemetry. The isolated subprocess regression test
demonstrates deferred reporting during a panic without mounting storage or
sending data to Azure:

```bash
GOEXPERIMENT=systemcrypto go test -v ./internal/telemetry \
  -run '^TestPanicDoesNotReportCleanShutdown$'
```

## Measurements and interpretation

- HTTP response counts include successful and failed status codes at the
  instrumented storage transport boundary. No-response failures are separate.
- SDK request/retry counters describe **azcore request scopes** and additional
  per-retry policy executions. They are not a physical-wire request count.
  Download RetryReader retries and BlobFuse-level retry loops are not included
  in the SDK retry count.
- Exact retry-backoff duration and confirmed retry-budget exhaustion are
  **deferred**, not reported as zero.
- Typed transport failures distinguish DNS, TLS/certificate, timeout, reset,
  cancellation, and other failures. Body-read errors have an additional count.
- Final FUSE callback outcomes are counted by operation, including native C
  cached reads/writes. EIO, EACCES, ENOSPC, and ESTALE retain their names; other
  Linux errno values are bounded numeric categories. Kernel-only errors are not
  visible, and not every callback result becomes an application syscall error.
- Effective approved configuration includes pipeline order/stream semantics,
  auth category, allow-other, component attribute TTL, separate kernel attribute
  timeout, platform/version information, runtime libfuse version where available,
  and negotiated FUSE protocol. The fingerprint hashes only these fields.

Failure categories are assigned only when evidence supports them. DNS is not
automatically a private-link failure, object absence is not container absence,
and unknown FUSE/helper failures are not guessed from raw stderr.

Snapshots use cumulative values within one session. Backend queries must
deduplicate events, order snapshots, and derive differences rather than sum
cumulative values. A snapshot can span multiple custom events: use snapshot ID,
part index, part count, sequence number, and session ID to reconstruct it.
Snapshots declare FUSE-counter availability. Counter encoding remains sparse:
an absent known counter means zero only for an available instrumented family in
a complete snapshot. SDK retry counters still describe azcore request scopes.
Exact retry backoff and confirmed exhaustion are not measured and must never
be treated as zero. HTTP counters observe Storage transport calls; FUSE
counters observe BlobFuse callbacks. These semantics are part of the schema
contract rather than repeated per-event encoding/scope/availability labels.

The payload no longer includes `observation_source`, `shutdown_outcome`,
`http_scope`, `fuse_observation_scope`, `retry_exhaustion_available`,
`mount_attempt_id`, `counter_encoding`, `retry_scope`, or
`exact_backoff_available`.
Use the recovery event type to identify later-start observations and
`termination_class` to interpret shutdown. Attribute-cache enablement is
derived from the presence of `attr_cache` in the comma-separated
`pipeline_components`, rather than a separate `attr_cache_enabled` property.
If the pipeline is unavailable, its cache enablement is unknown, not false.
Older retained events are dispatched once with these retired properties stripped,
preserving their event IDs, measurements, and terminal-cleanup association.
Their existing configuration fingerprints are retained rather than recomputed.

`mount_session_id` is the sole mount identity, including startup failure.
Daemon re-exec and state attachment reuse it, while a later mount creates a
new session ID. No separate attempt ID is generated, transferred or written
into new checkpoints. Recovery references the old session through
`previous_session_id` while retaining the reporting session's identity. Older
checkpoints containing an attempt field remain readable; session identity and
matching terminal-event evidence govern completion cleanup.

Network-failure counters increment on observed Storage transport errors and on
the first non-EOF read error for each observed response body. Classification
uses Go error types and wrapped causes, in this order: cancellation, DNS, typed
TLS/certificate failures, timeout, connection reset, then other. A DNS timeout
therefore counts as DNS, not both DNS and timeout. Body-read failure increments
both its category and the additional `transport_body_read` counter; do not sum
that counter with categories as independent failures. Repeated read errors on
the same body count once, and normal EOF does not count. Counters are assigned
to startup/runtime at observation time. HTTP status errors, such as a 503
response without a transport/read error, are not network-failure counters.

## Delivery limits

- Maximum encoded event: 32 KiB including the Application Insights envelope;
  snapshots split without silently truncating
  measurements.
- At most eight active sends per reporter, each in its own goroutine. Additional
  events are dropped immediately, without a waiting queue or lifecycle priority.
  At 32 KiB per event, active encoded payloads total at most 256 KiB, excluding
  temporary encoding, counters, goroutine, HTTP/TLS and other runtime overhead.
- A five-second request deadline and a five-second total shutdown wait for active
  sends. The detached parent's existing reporter remains
  cancellable during launch; normal parent exit cancels remaining delivery
  without adding a shutdown flush. A definitive launch failure can use the
  bounded flush only while collection is still permitted.
- Every ingestion failure, including transport errors, timeouts, HTTP 408/429/5xx
  and rejected partial responses, is diagnosed and dropped without retry.
  Retry-After is not used. Future events remain eligible. Redirects are not followed.
- Events older than seven days are discarded without sending. No exactly-once
  delivery guarantee is provided; one-shot dispatch can lose events.
- Reporting uses a separate HTTP client and is not counted as storage traffic.
  Delivery/persistence failures never become mount or filesystem-operation errors.

Limits apply to serialized payloads, not total Go/TLS memory consumption.

## Persistent state and recovery

By default, state is under
`<default-working-dir>/microsoft-telemetry`. This root must be owned by the
executing user and private (0700); state files are private (0600). A custom
directory must end in `microsoft-telemetry` to avoid treating an arbitrary
directory as a telemetry store.

Each logical mount has a private `mount-<opaque-id>` child directory. Its local
identity is an HMAC of the canonical mount path and mount-namespace context,
using a private, persistent random key in the root. Paths, scope labels, and the
key are never transmitted. Symlink aliases resolve to the same target. Daemon
parent/child and later starts in the same context reopen the same scope.
Opt-out and recovery operate only inside that scope, leaving other mounts' records
untouched. No destination partitioning is introduced: the deployment is expected
to use one common Microsoft ingestion destination.

For deployments replacing mount namespaces, optionally set a stable local
`microsoft-telemetry.mount-scope` label distinct for each logical mount. With
that label, identity uses the label plus canonical path instead of the transient
namespace. A surviving state root is still required, and process-death detection
remains conservative across PID namespaces. Changing the path/label selects a
different scope. Legacy unscoped records are diagnosed and retained, not
automatically replayed, migrated, or purged, because their mount ownership cannot
be reconstructed safely.

Retained state is bounded to 1 MiB and 512 recognized records per mount store, with
seven-day retention. Individual state records are independently capped at 64 KiB.
Normal writes stop at 960 KiB, reserving 64 KiB within the
same hard cap for recovery checkpoint updates. This is a logical allocation
budget, not a reservation of physical disk space. Temporary atomic-write files
and filesystem metadata can require additional space.

New storage contains session/checkpoint records, including transient terminal
evidence. Ordinary lifecycle events are not spooled to pending-event files.
Legacy pending-event files remain readable for one-shot consumption.
Recovery stores its event inside the existing session checkpoint and dispatches
that event directly; it does not allocate a redundant pending-event file.
Obsolete daemon-handoff baseline counters are removed only from confirmed-dead
sessions being converted to recovery checkpoints. Existing pending events and
terminal checkpoints are collected before updating unfinished
sessions. One failed update is diagnosed and does not hide other durable work.
This allows reporting and terminal retirement to release capacity even at the
record limit. Recovery still needs writable storage to persist a new observation;
physical disk failure is not made lossless by reserved logical capacity.

Before a lifecycle send, the store validates its session association and consumes
its retained evidence durably. Terminal consumption removes the matching session
checkpoint and synchronizes the directory before deleting any legacy pending copy.
Recovery consumption targets only `previous_session_id`, not the observer.
Ordinary attempted/successful-result events do not retire a running session.
Retained dispatch requires evidence still to exist, so concurrent startups cannot
both claim the same event. Conflicting evidence or failed durable consumption
prevents sending and produces a diagnostic; a later startup can try consumption
again because no network attempt was made.

Consumption precedes the network attempt and concurrency admission. A failed
send, a concurrency-limit drop or cancellation does not leave that lifecycle event
for another startup to resend. A crash between consumption and transmission can
lose the event. This is the intentional trade-off for send-once semantics.
Likewise, a clean stop can be absent remotely without generating a false later
unclean report. Unfinished running checkpoints still generate later-start
recovery; that recovery event itself has only one send opportunity.

Failed checkpoints or unavailable protected storage degrade reporting to
memory-only behavior and produce rate-limited diagnostics. No durable recovery
guarantee is possible if all terminal evidence could not be persisted.

Later startup checks host boot, PID namespace, PID, and process start identity
before declaring a previous process dead. Ambiguous or live sessions are not
classified as failed. Recovery is recorded as **unclean termination, cause
unknown**, with last-known uptime, not a confirmed crash/OOM or exact death time.
Cross-boot classification additionally requires matching local host identity
(a machine-id hash kept only in the local journal, never uploaded). Without it,
cross-boot ownership remains ambiguous. State is node-local, not a shared
cross-node coordination store.
Each startup creates at most eight new unfinished-session recovery records.
There is no reserved recovery queue. Recovery and live events share the eight
active-send slots; excess events are consumed and dropped rather than deferred.

Opt-out is per logical mount, including its recognized retained records from
earlier sessions. A command-line opt-out does not stop another running process.
The environment switch affects instances that inherit it, not all mounts
globally. Empty mount-scope directories and the root's private identity key are
retained for stable future ownership; per-store limits are not a host-wide quota.

There is no new supervisor, systemd hook, or independent reporter. After process
death nobody observes that unfinished session until a suitable later BlobFuse startup.
Container/CSI deployment owners must provision surviving writable state that a
later startup can discover. Node-local state can be lost on node deletion;
cross-node recovery is not provided. CSI NFS volumes do not run BlobFuse.

Production rollout requires approved collection/privacy policy, a destination,
backend capacity/retention/access controls, and measured overhead. The default
enabled policy alone does not configure or approve production ingestion.
