# Microsoft mount lifecycle telemetry

BlobFuse sends mount startup and observed shutdown events to Application Insights.
It does not collect periodic performance metrics or filesystem operation counters.

## Configuration and opt-out

Telemetry is enabled by default when a valid destination is configured.
Disable it with `--disable-telemetry`, `BLOBFUSE_DISABLE_TELEMETRY=true`, or:

```yaml
microsoft-telemetry:
  disabled: true
```

Any explicit disable wins. Settings apply at mount startup; remount to change
them. A background child also honors its parent's startup opt-out decision.
Disabling telemetry cannot withdraw events already received.

`BLOBFUSE_TELEMETRY_CONNECTION_STRING` is the only destination override. Its
ingestion URL must use HTTPS, a regional
`<region>.in.applicationinsights.azure.com` hostname, and port 443 (explicit or
default), with no credentials, custom path, query or fragment. Other hosts,
legacy/global endpoints and sovereign-cloud endpoints are rejected.
An invalid destination leaves telemetry inactive without failing the mount.
The hostname check does not authenticate publishers or limit which resource the
instrumentation key selects.

## Events and interpretation

| Event | Meaning | Measurement |
|---|---|---|
| `blobfuse.mount.attempted` | Startup began after configuration policy was readable. | None |
| `blobfuse.mount.result` | FUSE became ready or startup failed. | `time_to_mount_ms` or `time_to_failure_ms` |
| `blobfuse.mount.stopped` | Shutdown was observed. | `final_uptime_seconds` or `last_known_uptime_seconds` |

Events share a mount session ID. Attempted records the original startup time,
even when dispatched later by a background child. FUSE readiness does not
guarantee subsequent Storage access. Startup failures include a bounded stage
and category, not raw error text.

Stopped events use `termination_class=clean` after successful pipeline teardown,
or `unclean_unknown` when successful teardown was not established.
**Missing stopped events and `unclean_unknown` do not prove a crash or OOM kill.**
Dashboards describe observed outcomes, not every mount's final state.

## Collected data and privacy

Metadata includes BlobFuse version, pipeline/stream mode, authentication method,
distribution/kernel information, FUSE versions and attribute-cache timeouts.
No file contents, names/paths, account/container names, credentials, raw
configuration, diagnostic logs or raw error text are uploaded.

## Delivery and limitations

Each event is at most 32 KiB and gets one asynchronous send attempt, with no
retry, queue or local persistence. Network/HTTP failures are silently discarded;
invalid payloads or configuration can produce local diagnostics.

Requests have a five-second deadline. Normal shutdown waits at most five seconds
for sends to finish, then cancels outstanding requests. Successful background
startup adds no telemetry delivery wait.

Delivery and ordering are not guaranteed. Process termination can lose events;
later mounts do not recover or resend them. See the
[implementation notes](microsoft-telemetry-design.md) for lifecycle and test details.
