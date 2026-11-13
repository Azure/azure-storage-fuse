# BlobFuse Microsoft telemetry - release design

## Phase 1: in-process mount lifecycle only

The first release implements attempted, result and stopped events; approved
effective configuration; disable-wins policy; daemon identity handoff; typed
startup failures; and send-once-and-continue delivery.
It has no recovery event, local state store, checkpoint, retention scanner,
scope key, periodic snapshot timer, SDK/HTTP counter hooks or native operation
counter wrappers. See [the configuration and limitations](microsoft-telemetry.md).

An event has a random event ID and mount session ID. Background parent/child
reuse session identity and monotonic startup timing. Application Insights uses
`EventData` version 2 with BlobFuse `schema_version` 1.

For background mounts, the parent records but defers the attempted event, handing
its original timestamp to the daemon child. The child owns all three lifecycle
events and starts the attempted send during telemetry initialization. Successful
parent exit adds no delivery wait and cannot cancel the child's request.
Failures before handoff leave attempted and failure-result delivery with the
parent. A child dying before telemetry initialization can still lose the attempt.

Each lifecycle transition starts one deadline-bound HTTP goroutine. Guarded
lifecycle production bounds each process to at most three events; no separate
concurrency-admission counter or waiting queue is needed. There is no delivery
retry, persistent spool or replay. Delivery failures are discarded silently.
Requests have a five-second deadline and normal shutdown a five-second maximum
wait, ending early on completion or cancellation without an unbounded join.
Background send workers do not use the logger.
Events are at most 32 KiB. Delivery failure never disables future collection.
Live opt-out is sticky and emits no event. Its initialization owner is published
before dispatch, and effective policy is rechecked before reporting starts.

Session state uses one mutex and an explicit lifecycle:
`initializing -> collecting -> closing -> closed`, with a `detached` parent
state between collection and closing. The same lock protects configuration,
readiness, teardown and event construction/registration; callbacks recheck
eligibility after taking it. Shutdown releases the lock before the bounded
delivery wait. Concurrent close callers share a completion channel so none
returns before final-event construction and the bounded wait have finished.
Sticky disable remains atomic and cancels networking before acquiring the
session lock. Reporter worker synchronization remains separate.

Explicit successful component teardown is required for a clean stopped event.
Incomplete teardown observed during cleanup is `unclean_unknown`; abrupt process
death may produce no final event at all. There is no later-start diagnosis.
The teardown marker is the only shutdown classification input; telemetry does
not consume pipeline errors or change command return/exit behavior. Dashboard
counts describe observed shutdown outcomes only; a missing event is not evidence
of an unclean shutdown or crash.

## Initialization and daemon handoff

Initialization publishes a cancellable policy owner but hides it from collectors
until policy reconciliation completes. The policy callback runs outside the
session mutex; disable can finish while that callback is still running, and the
callback's eventual return cannot reactivate the session.

Handoff carries the session ID, monotonic startup clock, original attempt
timestamp and sticky parent opt-out decision. The launch environment is rebuilt
before each daemonization attempt, replacing stale handoff entries. The child
combines inherited disable with local YAML/CLI/environment policy: any disable
wins. A missing parent collector or invalid enabled handoff metadata cannot
create a new enabled child collector; invalid metadata produces a warning.

Handoff is a launch-time snapshot, not post-launch policy IPC. Later config
changes must be observed by the child's own watcher. Disables observed only by
the parent after launch are not communicated through this snapshot.

## Payload validation and delivery

Property names are allow-listed. Measurement names are allow-listed per event,
and their numeric values must be finite. Unknown names, measurements on the
wrong event and invalid values reject the whole event through the existing
invalid-payload diagnostic. Encoding enforces the 32-KiB envelope limit.

A foreground invocation or daemon child produces at most attempted, result and
stopped. A successfully detached parent sends none; a parent failing before
handoff sends at most attempted and failure result. One-time lifecycle guards
prevent repeated callbacks from registering duplicate events. Revisit this bound
before introducing periodic producers.

Each HTTPS request contains one event. HTTP 206 is rejected without reading its
body; there is no partial-batch handling or retry. Workers never log or access
session metadata. Completion bookkeeping supports the bounded shutdown wait
and eventual HTTP resource cleanup, not a concurrency-admission policy.

## Storage startup failure classification

ADLS validation probes DFS and Blob concurrently, retaining DFS error precedence.
Classification runs only after the final validation error is selected, so a
losing probe cannot overwrite the chosen category. `storageValidationError`
preserves exact existing user-facing messages and typed causes through
`Unwrap()`. Raw errors and endpoint details are not uploaded.

## Test isolation

The Go test-binary guard suppresses the compiled destination, but does not cover
separately launched production executables. Ordinary Go suites that launch the
binary set `BLOBFUSE_DISABLE_TELEMETRY=true` once at suite setup. The shared
ordinary loopback launcher sets it in the child environment, overriding inherited
enablement without changing the parent environment.

The PR CI job and nightly, coverage and release pipelines set opt-out once at
their job/pipeline boundaries. Shared step templates and child processes inherit
it. New pipelines consuming those templates must establish the same boundary.
This runtime setting does not change the default compiled into release binaries.

Standalone test scripts and benchmark launchers retain explicit opt-out because
they can run outside CI; scripts with repeated mounts export it once at entry.
Distributed-cache test deployments and cloned test pods disable collection
inside the container because host environment variables do not automatically
propagate to Kubernetes pods.

Telemetry-specific tests explicitly enable collection and supply local HTTPS
receivers. The opt-in real-mount regressions cover live opt-out and background
event ownership without Azure credentials or external ingestion. See the
[verification instructions](../test/telemetry_smoke/README.md).

## Deferred scope

Recovery, durable telemetry state, cumulative snapshots and operation counters
are outside Phase 1. Any future recovery implementation needs validation of
state ownership, terminal completion and retention for long-lived mounts.
Exact SDK retry-wait time and confirmed retry exhaustion are not implemented.
Existing telemetry files from earlier development builds are left untouched:
Phase 1 does not scan, replay, purge or migrate them.

## Release prerequisites

Production needs approved default-on/privacy policy, a production destination
replacing the compiled TEST resource, measured overhead, backend queries and
deployment coverage. In-process reporting cannot guarantee crash/OOM attribution
or delivery after process death.
The test destination is defined in `cmd/telemetry.go` and must be replaced before
merging to main or releasing. Default enablement alone is not production approval.
