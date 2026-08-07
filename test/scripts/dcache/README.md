# dist_cache nightly E2E helper scripts

These scripts stand up a local [kind](https://kind.sigs.k8s.io/) cluster,
deploy the [Tachyon](https://github.com/Azure/Tachyon) cache-server via its
Helm chart, and deploy blobfuse2 in-cluster for the dedicated `dcache_e2e`
suite.

The required subset is invoked from `azure-pipeline-templates/dist-cache-e2e.yml`
in the nightly build, but the scripts are self-contained enough to run locally
for iterative debugging.

## Files

| File | Purpose |
|---|---|
| `config/nightly.config`     | Shared bash config: kind and Kubernetes versions, cluster shape, image coordinates, and namespace. Every value is overridable via env var. |
| `install-prereqs.sh`        | Idempotently install docker-ce, the configured kind version, kubectl, and helm. |
| `setup-kind.sh`             | Create the kind cluster, label worker nodes for cache-server scheduling, and prepare `/var/lib/ssd/cacheserver` on every node container via `docker exec`. |
| `deploy-tachyon.sh`         | Install the `cache-server-prereq` chart, then the `cache-server` chart, both pulled directly from an OCI-enabled ACR (`oci://...`); imports the image directly into each kind node's containerd store. |
| `deploy-blobfuse2.sh`       | Render `docker/k8s/blobfuse2-dist-cache-deployment.yaml.tmpl` with storage credentials + image ref, side-load the blobfuse2 image into every kind node, apply, and wait for rollout. |
| `teardown-kind.sh`          | Delete the cluster. Best-effort (runs under `set +e`). |

## Local usage

```bash
# 1. Install prerequisites (one-time).
./test/scripts/dcache/install-prereqs.sh

# 2. Set the image + chart coordinates. The chart is pulled from an
#    OCI-enabled ACR; no source checkout of vienna-tachyon is required.
export CACHE_SERVER_IMAGE_REGISTRY=<acr-name>.azurecr.io
export CACHE_SERVER_IMAGE_REPO=cache-server
export CACHE_SERVER_IMAGE_TAG=<image-tag>

# Chart registry defaults to CACHE_SERVER_IMAGE_REGISTRY; override if the
# chart is in a different ACR.
# export CACHE_SERVER_CHART_REGISTRY=<other-acr>.azurecr.io
export CACHE_SERVER_CHART_REPO=charts/cache-server
export CACHE_SERVER_PREREQ_CHART_REPO=charts/cache-server-prereq
export CACHE_SERVER_CHART_VERSION=<chart-version>

# If the ACR is private, log in first (deploy-tachyon.sh does NOT do this):
#   az acr login --name <acr-name>
# or
#   helm registry login <acr-name>.azurecr.io -u <user> -p <password>

# 3. Bring up the cluster and deploy Tachyon.
./test/scripts/dcache/setup-kind.sh
./test/scripts/dcache/deploy-tachyon.sh

# 4. Deploy the in-cluster blobfuse2 pod driven by test/dcache_e2e/.
export BLOBFUSE2_IMAGE=<registry>/azure-blobfuse2:<tag>
export STO_ACC_NAME=<account>
export STO_ACC_KEY=<key>
export containerName=<container>
./test/scripts/dcache/deploy-blobfuse2.sh

# 5. Run the distributed-cache-specific tests.
go test -v -tags=fuse3 ./test/dcache_e2e/...

# 6. Teardown the cluster.
./test/scripts/dcache/teardown-kind.sh
```

## Notes

- The kind cluster shape (4 nodes: 1 control-plane + 3 workers) is set by
  `KIND_NODES` in `config/nightly.config`. `setup-kind.sh` generates the
  `kind create cluster --config=...` file on the fly from that value.
- The kind binary and node image are pinned by `KIND_VERSION` (default
  `v0.32.0`) and `KIND_NODE_IMAGE` (default `kindest/node:v1.36.1`). Update
  them together when upgrading Kubernetes. k8s 1.35+ requires cgroup v2 on
  the host.
- `deploy-tachyon.sh` sets `cacheServer.scheduler.enabled=false` because
  blobfuse2 E2E tests do not need the scheduler component.
- Nothing here needs `MINIKUBE_HOME` or `/mnt/minikube`. kind stores node
  container state under Docker's data-root -- if `/` is tight on the agent,
  point Docker's data-root at `/mnt/docker` via `/etc/docker/daemon.json`.
