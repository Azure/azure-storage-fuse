# Microsoft mount lifecycle telemetry - Phase 1

Phase 1 sends mount attempted/result/stopped custom events directly to
Application Insights. Later-start recovery, local telemetry persistence,
periodic snapshots and operation counters are not included.

> Development notice: the current implementation uses a test telemetry
> destination and is not approved for production rollout.

## Configuration and opt-out

Telemetry is default-enabled when a destination exists.
`BLOBFUSE_TELEMETRY_CONNECTION_STRING` is the only destination override.
Any explicit disable wins: `microsoft-telemetry.disabled` in YAML,
`--disable-telemetry`, or `BLOBFUSE_DISABLE_TELEMETRY=true`.

The ingestion URL must use HTTPS and an Azure Public regional hostname of the
form `<region>.in.applicationinsights.azure.com`, with no custom path, query or
fragment. Only the default HTTPS port or explicit port 443 is accepted.
Non-Microsoft collectors, localhost, legacy/global endpoints and sovereign-cloud
endpoints are not supported. An invalid destination leaves telemetry inactive;
it does not fail the mount. This hostname restriction does not restrict which
Application Insights resource the instrumentation key identifies.

```yaml
microsoft-telemetry:
  disabled: false
```

Telemetry settings are evaluated at mount startup. Changes to watched YAML do
not affect an existing mount, including its eventual stopped event. Unmount and
mount again to change telemetry policy. Background children inherit the parent's
startup opt-out decision and also apply their own startup policy; any disable
wins. Disabling telemetry cannot withdraw events already received.

## Events and interpretation

| Event | Meaning |
|---|---|
| `blobfuse.mount.attempted` | Startup began after configuration policy was readable. |
| `blobfuse.mount.result` | Readiness or definitive startup failure, with elapsed startup duration and available approved configuration. |
| `blobfuse.mount.stopped` | `clean` or `unclean_unknown` termination, with final/last-known uptime. |

Only the following measurements are reported:

| Event | Allowed measurements |
|---|---|
| `blobfuse.mount.attempted` | None |
| `blobfuse.mount.result` | `time_to_mount_ms`, `time_to_failure_ms` |
| `blobfuse.mount.stopped` | `final_uptime_seconds`, `last_known_uptime_seconds` |

Events from one mount share a session ID. The attempted event records the
original startup time, including when a background mount sends it later.
Readiness means FUSE initialization completed; it does not guarantee later
Storage access will succeed. Startup failure events report a bounded failure
stage and category, not raw error text.

`clean` means pipeline teardown completed successfully. `unclean_unknown` means
cleanup was observed without establishing successful teardown; it does not
confirm a crash, OOM kill or host failure.

Dashboards should describe **observed shutdown outcomes**, not all mount outcomes.
Abrupt process death or delivery failure can leave no stopped event. A missing
event must not be counted as `unclean_unknown` or assumed to indicate a crash.

## Collected data and privacy

Approved metadata includes pipeline order/stream mode, auth method,
distribution/kernel information, FUSE API/library versions,
and component/kernel attribute timeouts.
No file contents, names/paths, account/container names, credentials, raw
configuration, logs or raw error text are uploaded. Existing diagnostic logs
remain separate from this payload.

## Delivery and limitations

Each event gets one asynchronous delivery attempt, with no retry or persistent
queue. Network/HTTP failures are silently discarded and do not prevent later
events from being attempted. Invalid telemetry payloads or configuration can
produce local diagnostics. The maximum encoded event is 32 KiB.
Requests have a five-second deadline. Normal session shutdown waits at most
five seconds for active sends, ending early when sends complete. Requests still
in flight when the shutdown budget expires are canceled. Successful background
startup adds no telemetry delivery wait.
Delivery and event ordering are not guaranteed.

Telemetry is not persisted locally. Process termination can lose in-flight and
final lifecycle events; a later mount does not diagnose the earlier termination
or resend its events.

Implementation, test-isolation and release-readiness notes are in the
[telemetry design document](microsoft-telemetry-design.md).
