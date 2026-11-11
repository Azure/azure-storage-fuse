# Microsoft mount lifecycle telemetry - Phase 1

Phase 1 sends mount attempted/result/stopped/recovery custom events directly to
Application Insights. Snapshot reporting, periodic timers, filesystem operation
counters and SDK/HTTP/network counters are deferred to Phase 2.

## Configuration and opt-out

Telemetry is default-enabled when a destination exists. The compiled destination
in cmd/telemetry.go currently points to TEST and must be replaced with an approved
production resource before merging to main or releasing.
BLOBFUSE_TELEMETRY_CONNECTION_STRING is the only destination override.
Any explicit disable wins: microsoft-telemetry.disabled in YAML,
--disable-telemetry, or BLOBFUSE_DISABLE_TELEMETRY=true.
Live watched-YAML disable cancels delivery and purges this logical mount's
recognized retained state; re-enabling requires a new mount.

```yaml
microsoft-telemetry:
  disabled: false
  # state-directory: /persistent/blobfuse/microsoft-telemetry
  # mount-scope: stable-volume
```

## Events and interpretation

| Event | Meaning |
|---|---|
| blobfuse.mount.attempted | Startup began after configuration policy was readable. |
| blobfuse.mount.result | Readiness or definitive startup failure, with elapsed startup duration and available approved configuration. |
| blobfuse.mount.stopped | clean, runtime_error or unclean_unknown termination, with final/last-known uptime. |
| blobfuse.mount.recovery | Later startup observed an unfinished previous session, with previous readiness and last recorded uptime. |

Session identity is shared across daemon parent/child; there is no separate
attempt ID. Clean termination requires explicit successful pipeline teardown.
Panic cleanup does not imply clean termination. Readiness is the FUSE
initialization boundary, not a guarantee of later Storage success.
Recovery means unclean_unknown, not confirmed crash/OOM. Without periodic
snapshots the saved runtime uptime can be stale; recovery reports only the
available checkpoint, never an invented exact lifetime.

Approved configuration includes pipeline order/stream mode, auth method,
distribution/kernel information, FUSE API/library/protocol versions,
allow_other and component/kernel attribute timeouts. Its fingerprint hashes
only these approved fields. No file contents, names/paths, account/container
names, credentials, raw configuration, logs or raw error text are uploaded.

## One-shot delivery and recovery

Each event gets one asynchronous send opportunity, with up to eight active
sends per reporter and no waiting queue. Excess events drop immediately.
Every network/HTTP failure is diagnosed and dropped without retry; future
events remain eligible. The maximum encoded event is 32 KiB. Requests have a
five-second deadline; shutdown waits at most five seconds for active sends
before cancellation. Disk work is synchronous and not part of that network budget.

Private state is scoped per canonical mount path and mount namespace, or a
stable deployment label, using a local private HMAC key. Raw scope identifiers
are not uploaded. The state directory must end in microsoft-telemetry, be
private (0700), and belong to the executing user; files are private (0600).
State is bounded to 1 MiB/512 recognized records per logical mount, seven-day
retention, and 64 KiB per record. Ordinary writes stop at 960 KiB, reserving
64 KiB within the hard cap for recovery; this does not reserve physical disk.

Checkpoints preserve local process ownership, readiness and terminal evidence.
Legacy pending files can be consumed once. Before terminal/recovery dispatch,
evidence is validated and durably consumed, even when concurrency admission
drops the event. A crash between consumption and sending can lose it. Failed
consumption prevents network delivery and is diagnosed.
Concurrent startups cannot claim consumed evidence twice. Failed sends are
never replayed on later startup; unfinished running checkpoints can still
produce a new recovery observation. Process identity uses boot, namespace, PID
and process start evidence rather than PID alone. Cross-boot inference requires
matching local host evidence. At most eight new recovery markers are created
per pass; full-record stores update existing checkpoints rather than allocating
another event file.

State repair is retried after ordinary stale-mount cleanup, without recreating
session identity or resending in-flight events. Opt-out cleanup can also retry
after path repair, without starting a reporter. No independent supervisor
reports after process death. Ephemeral/node-deleted state cannot be recovered.
Legacy unscoped records remain untouched because ownership is unknown.
