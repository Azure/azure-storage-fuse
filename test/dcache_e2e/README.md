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
| `metrics_test.go` | In-pod Prometheus scraper (`kubectl exec ... curl`) and `CacheServerMetrics` delta helpers. |
| `read_path_test.go` | `TestReadPath_L2MissPopulatesAndHits` — the canonical L2 miss → populate → hit test. |

## Running locally

Prerequisites (in addition to the standard `blobfuse2` build):

1. A kind cluster with Tachyon deployed and its wire port reachable on
   `localhost` via `kubectl port-forward`. Use
   [test/scripts/dcache/setup-kind.sh](../scripts/dcache/setup-kind.sh) +
   [deploy-tachyon.sh](../scripts/dcache/deploy-tachyon.sh) +
   [expose-cacheserver.sh](../scripts/dcache/expose-cacheserver.sh).
   Prometheus scrapes are done in-cluster via `kubectl exec ... curl`,
   so no separate metrics port-forward is required.
2. A running blobfuse2 Deployment in the kind cluster. Deploy it with
   [test/scripts/dcache/deploy-blobfuse2.sh](../scripts/dcache/deploy-blobfuse2.sh),
   which renders
   [docker/k8s/blobfuse2-dist-cache-deployment.yaml.tmpl](../../docker/k8s/blobfuse2-dist-cache-deployment.yaml.tmpl)
   with your storage credentials + image ref, side-loads the image into
   every kind node, and waits for rollout. The defaults expect deployment
   `blobfuse2-dist-cache` in namespace `blobfuse2-dist-cache`, selected
   by `app=blobfuse2-dist-cache`, with its FUSE mount at
   `/mnt/blobfuse_mnt` inside the container.
3. An Azure Storage account + container the test can seed data into.

```bash
# From the repo root, with blobfuse2 already built.

export DCACHE_SERVERS=$(cat /tmp/dcache_server_list.txt)

# Seed the credentials the test uses to upload payloads out-of-band AND
# that deploy-blobfuse2.sh bakes into the pod's config Secret.
export STO_ACC_NAME=<account>
export STO_ACC_KEY=<key>
export STO_ACC_ENDPOINT=https://<account>.blob.core.windows.net
export containerName=<container>

# Build (or pull) the blobfuse2 container image, then point the deploy
# script at it. For a locally-built image, set BLOBFUSE2_IMAGE_PULL=false
# to skip the `docker pull` step.
export BLOBFUSE2_IMAGE=<registry>/azure-blobfuse2:<tag>
./test/scripts/dcache/deploy-blobfuse2.sh

# Run the read-path test.
go test -v -tags=fuse3 ./test/dcache_e2e/... \
  -run '^TestReadPath_L2MissPopulatesAndHits$' -args \
  -pod-namespace=blobfuse2-dist-cache \
  -pod-deployment=blobfuse2-dist-cache \
  -pod-selector=app=blobfuse2-dist-cache \
  -pod-mount-path=/mnt/blobfuse_mnt \
  -kubectl-bin=kubectl
```

Flags fall back to the environment variables shown above (matching the
names the existing e2e pipeline already sets), so a fully-populated env
lets you drop most of the `-args` list.

If pod discovery or the in-pod `curl` fails, the metric-based
assertions are skipped and only data integrity is enforced. Override the
pod-local Prometheus port with `-cacheserver-metrics-port` if your
Tachyon Helm chart exposes it somewhere other than 9096.

## Pipeline integration

Wired from
[azure-pipeline-templates/dist-cache-e2e.yml](../../azure-pipeline-templates/dist-cache-e2e.yml).
For each mount variant the template already exercises via
`e2e-tests.yml`, an additional `dcache_e2e Go tests` step runs the tests
in this package against the same generated config.
