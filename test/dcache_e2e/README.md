# dist_cache E2E tests (`test/dcache_e2e/`)

Read-path E2E tests focused on the `dist_cache` component. These tests assert
`dist_cache` *behaviour* — L2 miss populates, L2 hit serves the same bytes
we wrote, and cache-server metrics move in the expected direction.

## Design

* **The mount is read-only.** The reference pod config sets `read-only: true`,
  and every test verifies the cloned pod reports `ro` in `/proc/mounts` before
  reading. Payloads are uploaded directly to Azure Storage through the Azure SDK.
* **Each test owns an isolated Deployment.** A test clones the reference
  blobfuse2 Deployment with a unique name and selector, then deletes it in
  cleanup. Tests that need to clear local `block_cache` restart their clone.
* **Behaviour + metrics.** Data integrity is asserted by MD5 on every
  read. Cache behaviour is asserted by diffing Prometheus counters from
  the Tachyon cache-server pods between snapshots. When metric endpoints
  are not configured (local iteration), the metric assertions are logged
  as skipped and the MD5 assertion still runs.

## Layout

| File | Purpose |
|---|---|
| `main_test.go` | `TestMain`, flag registration, per-run config. |
| `pod_mount_test.go` | Per-test Deployment cloning, scaling, restart, and mount-read helpers. |
| `helpers_test.go` | Random payload, MD5, and Azure SDK upload/delete helpers. |
| `metrics_test.go` | In-pod Prometheus scraper (`kubectl exec ... curl`) and `CacheServerMetrics` delta helpers. |
| `metrics_parse_test.go` | Unit tests for Prometheus parsing and metric deltas. |
| `read_path_test.go` | `TestReadPath_L2MissPopulatesAndHits` — the canonical L2 miss → populate → hit test. |
| `node_failure_test.go` | Stops a kind worker hosting one cache-server, verifies mixed L2/Azure fallback, then restores the node. |
| `stampede_test.go` | Verifies concurrent cold reads coalesce into one Azure GET. |
| `warm_cache_test.go` | Verifies late-joining pods read previously populated data from L2. |

## Running locally

Prerequisites (in addition to the standard `blobfuse2` build):

1. A kind cluster with Tachyon deployed. Use
   [test/scripts/dcache/setup-kind.sh](../scripts/dcache/setup-kind.sh) +
  [deploy-tachyon.sh](../scripts/dcache/deploy-tachyon.sh).
  The pod suite uses in-cluster discovery and scrapes Prometheus through
  `kubectl exec`.
2. A running blobfuse2 Deployment in the kind cluster. Deploy it with
   [test/scripts/dcache/deploy-blobfuse2.sh](../scripts/dcache/deploy-blobfuse2.sh),
   which renders
   [docker/k8s/blobfuse2-dist-cache-deployment.yaml.tmpl](../../docker/k8s/blobfuse2-dist-cache-deployment.yaml.tmpl)
   with your storage credentials + image ref, side-loads the image into
   every kind node, and waits for rollout. The defaults expect deployment
  `blobfuse2-dist-cache` in namespace `blobfuse2-dist-cache`, with its FUSE mount at
   `/mnt/blobfuse_mnt` inside the container.
3. An Azure Storage account + container the test can seed data into.

```bash
# From the repo root, with blobfuse2 already built.

# Seed the credentials the test uses to upload payloads out-of-band AND
# that deploy-blobfuse2.sh bakes into the pod's config Secret.
export STO_ACC_NAME=<account>
export STO_ACC_KEY=<key>
export STO_ACC_ENDPOINT=https://<account>.blob.core.windows.net
export containerName=<container>

# Build (or pull) the blobfuse2 container image, then point the deploy
# script at it.
export BLOBFUSE2_IMAGE=<registry>/azure-blobfuse2:<tag>
./test/scripts/dcache/deploy-blobfuse2.sh

# Run the read-path test.
go test -v -tags=fuse3 ./test/dcache_e2e/... \
  -run '^TestReadPath_L2MissPopulatesAndHits$' -args \
  -pod-namespace=blobfuse2-dist-cache \
  -pod-deployment=blobfuse2-dist-cache \
  -pod-mount-path=/mnt/blobfuse_mnt \
  -kubectl-bin=kubectl
```

Flags fall back to the environment variables shown above (matching the
names the existing e2e pipeline already sets), so a fully-populated env
lets you drop most of the `-args` list.

The node-failure scenario requires access to the Docker daemon. It only kills
a container carrying kind's `io.x-k8s.kind.cluster` label, avoids the worker
hosting the active blobfuse2 pod, and restarts the node in `t.Cleanup`.

If pod discovery or the in-pod `curl` fails, the metric-based
assertions are skipped and only data integrity is enforced. Override the
pod-local Prometheus port with `-cacheserver-metrics-port` if your
Tachyon Helm chart exposes it somewhere other than 9096.

## Pipeline integration

Wired from
[azure-pipeline-templates/dist-cache-e2e.yml](../../azure-pipeline-templates/dist-cache-e2e.yml).
The template deploys the in-cluster dependencies and runs only this package.
