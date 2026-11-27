# Microsoft mount startup telemetry

BlobFuse sends mount startup results to Application Insights. It does not report
shutdown, mount uptime, periodic performance metrics or filesystem operation counters.

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

## Mount result

| Event | Meaning | Measurement |
|---|---|---|
| `blobfuse.mount.result` | One startup outcome: `1` for FUSE readiness, `0` for startup failure. | `result`, `mount_latency_ms` when timing is available |

Startup latency includes parent work for background mounts.
An early process exit can leave no result. FUSE readiness
does not guarantee subsequent Storage access. Failures include `failure_stage`,
`error_category` and, when recognized, `error_code`. A component start error
is reported as a failure even if FUSE later signals readiness.

Result counts describe observed outcomes, not total mount attempts.

## Collected data and privacy

Metadata includes version, pipeline, platform and Storage classifications.
Startup results also include approved settings the customer supplied
(`<component>.customer_provided_parameters.<setting>`) and values sized
automatically by BlobFuse (`dynamic_parameters.<component>.<setting>`). Fixed
defaults are omitted, though an automatic value can still land on a fixed floor
or cap. Values over 1,024 bytes are omitted. `config_source` (`yaml`, `cli`,
`env`, a combination, or `defaults`) describes only the reported customer
settings. Legacy `stream`, `lfuse` and `-o` inputs are reported under their
`stream` and `libfuse` names.

The configured account name is sent if it has a valid Storage account name
format, regardless of the endpoint hostname.
No file contents or paths, container names, credentials, endpoint URLs, raw
configuration, logs or raw error text are sent.

## Delivery and limitations

Each event is at most 32 KiB and gets one asynchronous send attempt, with no
retry, queue or local persistence. Network/HTTP failures are silently discarded;
invalid payloads or configuration can produce local diagnostics.

Requests have a five-second deadline. If the startup result is still being sent
when the mount process exits, BlobFuse waits at most five seconds, then cancels
it. Successful background startup adds no telemetry delivery wait.

Delivery is not guaranteed. Process termination can lose events;
later mounts do not recover or resend them. See the
[implementation notes](microsoft-telemetry-design.md) for implementation and test details.
