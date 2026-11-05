# Microsoft lifecycle and error telemetry

BlobFuse can collect structured mount lifecycle events, approved effective
configuration, and aggregate error/retry counters for a Microsoft-owned
Application Insights resource. This is separate from local diagnostic logging
and health-monitor output. It does not collect file contents, paths, account or
container names, credentials, raw configuration, or raw error text.

## Enablement and opt-out

Telemetry policy is enabled by default, but this build has **no embedded
Application Insights destination**. Without a configured destination, collection,
telemetry persistence, and delivery are inactive and a local warning is emitted.
Deployment must supply a BlobFuse-owned, approved destination; do not reuse
AzCopy's resource.

```yaml
microsoft-telemetry:
  disabled: false
  connection-string: "InstrumentationKey=<key>;IngestionEndpoint=https://<ingestion-host>/"
  snapshot-interval-sec: 300
  # Optional. Must be a dedicated private directory ending in microsoft-telemetry.
  # state-directory: /persistent/blobfuse/microsoft-telemetry
```

The connection string can also be supplied with
`BLOBFUSE_TELEMETRY_CONNECTION_STRING` or `--telemetry-connection-string`.
Normal configuration/environment/CLI precedence applies to the destination.

For explicitly opted-in development testing, the repository includes a personal
test resource in `test/scripts/telemetry-test.env`. From the repository root:

```bash
source test/scripts/telemetry-test.env
```

This is not a production default and is not automatically loaded by BlobFuse or
CI. Its instrumentation key is a public routing identifier, not sender
authentication. Use synthetic test traffic only.

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
telemetry records in the configured store; failures to access/delete that store
are diagnosed locally. Files outside that dedicated store are not deleted.
Changing the watched configuration to disabled cancels reporting and purges the
active store. Re-enablement and other telemetry-setting changes require a new
mount. Diagnostic logging and storage authentication are unchanged.

## Records and cadence

| Record | Purpose |
|---|---|
| `blobfuse.mount.attempted` | One actual instrumented mount attempt |
| `blobfuse.mount.result` | Confirmed FUSE readiness or definitive startup failure |
| `blobfuse.mount.snapshot` | Session-cumulative HTTP/SDK/transport/FUSE counters and uptime |
| `blobfuse.mount.stopped` | Graceful return/teardown outcome and final known uptime |
| `blobfuse.mount.recovery` | Later discovery of an unfinished previous session |

Collection starts after configuration can be read and telemetry policy resolved.
Failures before that boundary (including invalid configuration syntax or a
loader failure) cannot be reported by this implementation.

Lifecycle records are queued promptly. After readiness, snapshots are taken every
300 seconds with up to 10% jitter. The configurable interval must be between
60 and 3600 seconds. A final snapshot is queued when mounted execution returns.
Only lifecycle records are durably replayed; periodic/final counter snapshots
are memory-only.

Background mounts share attempt/session identity across the launching parent
and daemon. Startup counters are transferred through protected state where
available. Startup elapsed time uses the host monotonic clock across re-exec;
uptime starts at confirmed FUSE initialization. Readiness is not a guarantee that
every future storage operation will succeed. A startup wait timeout is not
classified as a definitive failure.

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
Snapshots declare observation scopes, FUSE-counter availability, and sparse
encoding: an absent known counter means zero only for an available instrumented
family in a complete snapshot. Deferred exact-backoff/exhaustion metrics have
explicit unavailable flags and must never be treated as zero.

## Delivery limits

- Maximum encoded event: 8 KiB; snapshots split without silently truncating
  measurements.
- Maximum queue: 256 KiB of payloads and 256 records, plus bounded record
  metadata and one in-flight event. Lifecycle records take priority over snapshots.
- One concurrent send, a five-second attempt deadline, and a five-second total
  shutdown flush budget. Daemon handoff cancels parent delivery without waiting
  for the shutdown budget.
- Transient failures use exponential backoff/jitter and Retry-After with bounded
  age. Permanent ingestion rejections are diagnosed and discarded. Redirects
  are not followed.
- Event IDs and payloads survive retransmission. An acknowledgement can be lost
  after acceptance; Application Insights does not provide exactly-once ingestion.
- Reporting uses a separate HTTP client and is not counted as storage traffic.
  Delivery/persistence failures never become mount or filesystem-operation errors.

Limits apply to serialized payloads, not total Go/TLS memory consumption.

## Persistent state and recovery

By default, state is under
`<default-working-dir>/microsoft-telemetry`. The directory must be owned by the
executing user and private (0700); state files are private (0600). A custom
directory must end in `microsoft-telemetry` to avoid treating an arbitrary
directory as a telemetry store.

Retained state is bounded to 1 MiB and 512 recognized records per store, with
seven-day retention. Storage contains session/checkpoint records and pending
lifecycle events. Completed session state is removed after its terminal event
is acknowledged. Failed checkpoints or unavailable protected storage degrade
reporting to memory-only behavior and produce rate-limited diagnostics.

Later startup checks host boot, PID namespace, PID, and process start identity
before declaring a previous process dead. Ambiguous or live sessions are not
classified as failed. Recovery is recorded as **unclean termination, cause
unknown**, with last-known uptime, not a confirmed crash/OOM or exact death time.
Cross-boot classification additionally requires matching local host identity
(a machine-id hash kept only in the local journal, never uploaded). Without it,
cross-boot ownership remains ambiguous. State is node-local, not a shared
cross-node coordination store.
Concurrent startups may retransmit the same persisted event; stable IDs allow
backend deduplication.
Each startup creates at most eight new unfinished-session recovery records.
Replay uses at most half the in-memory queue, reserving room for the current
mount. Additional retained work is explicitly diagnosed and deferred to a later
startup. Existing pending events are not rewritten merely to enqueue a retry.

Mounts with independent collection policies should use separate state
directories. Opt-out purges recognized retained records in its configured store.

There is no new supervisor, systemd hook, or independent reporter. After process
death nobody sends pending telemetry until a suitable later BlobFuse startup.
Container/CSI deployment owners must provision surviving writable state that a
later startup can discover. Node-local state can be lost on node deletion;
cross-node recovery is not provided. CSI NFS volumes do not run BlobFuse.

Production rollout requires approved collection/privacy policy, a destination,
backend capacity/retention/access controls, and measured overhead. The default
enabled policy alone does not configure or approve production ingestion.
