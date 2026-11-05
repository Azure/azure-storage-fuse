# VM-to-Application-Insights smoke test

This explicitly invoked workflow tests the actual local BlobFuse binary against
an existing storage account and a test Application Insights destination.
It is not started by `go test` or CI.

From the repository root:

```bash
GOEXPERIMENT=systemcrypto ./build.sh
GOTOOLCHAIN=local GOEXPERIMENT=systemcrypto CGO_ENABLED=1 \
  go run ./test/telemetry_smoke \
  --binary ./blobfuse2 \
  --config ./config.yaml \
  --env-file ./test/scripts/telemetry-test.env
```

The original configuration and storage credentials are never modified or printed.
The workflow:

1. Creates a private test directory and a temporary credential-bearing config.
2. Uses the existing storage configuration with a small file-cache pipeline,
   read-only mounting, no preload, and isolated telemetry state.
3. Starts a temporary loopback HTTPS verification relay. Only the child process
   trusts its certificate; normal Azure certificate roots remain available.
   The relay forwards the real BlobFuse event envelopes unchanged to the
   configured Application Insights endpoint and captures ingestion acknowledgements.
   This is a test fixture, not a production gateway.
4. Mounts with the actual binary, probes a random missing entry, and waits
   70 seconds for a one-minute snapshot (including jitter). No blob contents
   are downloaded, and no blobs are created, modified, or deleted.
5. Unmounts, waits for shutdown delivery, and verifies acknowledged attempted,
   result, snapshot, and stopped events.
6. Deletes the temporary credential-bearing config. Private logs and a sanitized
   `report.json` remain for review.

Forwarding is limited to **10 requests and 80 KiB of JSON payload**, with an
8-KiB per-request limit. HTTPS overhead is additional. Normal runs send about
five events; retries or snapshot splitting may change the count within the cap.
Cleanup retries unmount and uses lazy unmount/termination only for its own
private test mount/worker if necessary; any forced cleanup fails the test.
SIGINT/SIGTERM request cleanup; SIGKILL or host loss cannot guarantee cleanup.

Artifacts default to a fresh directory under
`$XDG_CACHE_HOME/blobfuse2-telemetry-smoke` (or the platform user-cache default).
`--output` selects another parent directory. Do not commit generated configs or
logs: local diagnostic logs may contain storage identifiers.

Acceptance responses establish that Azure ingestion acknowledged the events.
They do not independently verify indexing/query visibility. In the test
Application Insights resource's Logs view, use the session ID from `report.json`:

```kusto
customEvents
| where timestamp > ago(2h)
| where tostring(customDimensions.mount_session_id) == "<session-id>"
| project timestamp, name, customDimensions, customMeasurements
| order by timestamp asc
```

Fixture tests use only a local fake HTTPS server and send nothing to Azure:

```bash
GOEXPERIMENT=systemcrypto go test -race ./test/telemetry_smoke
```
