# BlobFuse Microsoft telemetry - two-phase design

## Phase 1: mount lifecycle and recovery

The first release implements attempted, result, stopped and recovery events;
approved effective configuration; disable-wins policy; daemon identity handoff;
protected per-mount state; typed startup failures; and send-once-and-continue
delivery. It has no snapshot event, timer, SDK/HTTP counter hooks or native
filesystem-operation wrappers. See microsoft-telemetry.md for its current contract.

An event has a random event ID and mount session ID. Background parent/child
reuse session identity and monotonic startup timing. Application Insights uses
EventData ver=2 with BlobFuse schema_version=1.

Lifecycle evidence is consumed before its sole send opportunity. Up to eight
HTTP goroutines run concurrently; excess work drops. There is no waiting queue,
retry timer or pending-event spool. Recovery detects definitely dead prior
owners on a later startup and consumes each observation before sending.
Failure never disables future collection; opt-out is sticky and emits no event.

## Phase 2: cumulative snapshots

The next release adds periodic/final cumulative SDK/HTTP/network and native
FUSE counters, sparse encoding, bounded snapshot splitting and snapshot metadata.
Snapshots remain memory-only and use the same one-shot delivery policy.
It also transfers parent startup-counter baselines to the daemon child.
Exact SDK retry-wait time and confirmed retry exhaustion are not implemented.

## Release prerequisites

Production needs approved default-on/privacy policy, a production destination
replacing the compiled TEST resource, measured overhead, backend queries and
deployment coverage. In-process reporting cannot guarantee crash/OOM attribution
or delivery after process death; recovery requires surviving local state.
