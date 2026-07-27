# dist_cache E2E tests (`test/dcache_e2e/`)

Read-path E2E tests focused on the `dist_cache` component. Complements
`test/e2e_tests/` (which is generic filesystem correctness under a
`dist_cache`-enabled config): the tests here specifically assert
`dist_cache` *behaviour* — L2 miss populates, L2 hit serves the same bytes
we wrote, cache-server metrics move in the expected direction.

## Design

* **Mount is read-only.** `dist_cache` is a read-only L2 in the current
  scope; writes never traverse it. The tests mount blobfuse2 with
  `--read-only=true` and seed all payloads directly to Azure Storage via
  the Azure SDK.
* **Test owns the mount lifecycle.** To distinguish an L2-hit read from a
  same-handle in-memory read, the test unmounts and remounts between reads
  so that `block_cache`'s cooked blocks are guaranteed gone.
* **Behaviour + metrics.** Data integrity is asserted by MD5 on every
  read. Cache behaviour is asserted by diffing Prometheus counters from
  the Tachyon cache-server pods between snapshots. When metric endpoints
  are not configured (local iteration), the metric assertions are logged
  as skipped and the MD5 assertion still runs.

## Layout

| File | Purpose |
|---|---|
| `main_test.go` | `TestMain`, flag registration, per-run config. |
| `mount.go` | Mount / unmount / remount helpers (owns the FUSE lifecycle). |
| `helpers.go` | Random payload + MD5 + Azure SDK upload / delete / download. |
| `metrics.go` | Prometheus text-format scraper and delta helpers. |
| `read_path_test.go` | `TestReadPath_L2MissPopulatesAndHits` — the canonical L2 miss → populate → hit test. |

## Running locally

Prerequisites (in addition to the standard `blobfuse2` build):

1. A kind cluster with Tachyon deployed and its pods reachable on
   `localhost` via `kubectl port-forward`. Use
   [test/scripts/dcache/setup-kind.sh](../scripts/dcache/setup-kind.sh) +
   [deploy-tachyon.sh](../scripts/dcache/deploy-tachyon.sh) +
   [expose-cacheserver.sh](../scripts/dcache/expose-cacheserver.sh) +
   [expose-metrics.sh](../scripts/dcache/expose-metrics.sh).
2. An Azure Storage account + container the test can seed data into.
3. A generated blobfuse2 config file with `DCACHE_SERVERS` and storage
   credentials substituted (see the
   [dist-cache-e2e.yml](../../azure-pipeline-templates/dist-cache-e2e.yml)
   pipeline template for the `gen-test-config` invocation).

```bash
# From the repo root, with blobfuse2 already built.

export DCACHE_SERVERS=$(cat /tmp/dcache_server_list.txt)
export DCACHE_METRICS_ENDPOINTS=$(cat /tmp/dcache_metrics_endpoints.txt)

# Seed the credentials the test uses to upload payloads out-of-band.
export STO_ACC_NAME=<account>
export STO_ACC_KEY=<key>
export STO_ACC_ENDPOINT=https://<account>.blob.core.windows.net
export containerName=<container>

# Generate a dist_cache blobfuse2 config from the template.
./blobfuse2 gen-test-config \
    --config-file=testdata/config/azure_key_dist_cache_block_e2e.yaml \
    --container-name="$containerName" \
    --temp-path=/tmp/blobfuse2_tmp \
    --output-file=/tmp/blobfuse2_dcache.yaml

mkdir -p /tmp/blob_mnt /tmp/blobfuse2_tmp

# Run the read-path test.
go test -v -tags=fuse3 ./test/dcache_e2e/... -args \
    -blobfuse-bin="$PWD/blobfuse2" \
    -config-file=/tmp/blobfuse2_dcache.yaml \
    -mnt-path=/tmp/blob_mnt \
    -tmp-path=/tmp/blobfuse2_tmp
```

Flags fall back to the environment variables shown above (matching the
names the existing e2e pipeline already sets), so a fully-populated env
lets you drop most of the `-args` list.

If `-dcache-metrics-endpoints` is empty, the metric-based assertions
are skipped and only data integrity is enforced. That mode is useful for
local iteration when you want to validate the mount / SDK plumbing
without also standing up the metrics port-forwards.

## Pipeline integration

Wired from
[azure-pipeline-templates/dist-cache-e2e.yml](../../azure-pipeline-templates/dist-cache-e2e.yml).
For each mount variant the template already exercises via
`e2e-tests.yml`, an additional `dcache_e2e Go tests` step runs the tests
in this package against the same generated config.
