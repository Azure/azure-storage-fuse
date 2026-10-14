# distributed_cache nightly E2E helper scripts

These scripts stand up a local [kind](https://kind.sigs.k8s.io/) cluster,
deploy the [DACS](https://github.com/Azure/dacs) cache-server via its
Helm chart, and deploy blobfuse2 in-cluster for the dedicated `dcache_e2e`
suite.

The required subset is invoked from `azure-pipeline-templates/dist-cache-e2e.yml`
in the nightly build, but the scripts are self-contained enough to run locally
for iterative debugging.

## Files

| File | Purpose |
|---|---|
| `config/nightly.config`     | Shared bash config: kind and Kubernetes versions, cluster shape, pinned DACS chart version, and namespace. Cluster settings are overridable via env var. |
| `install-prereqs.sh`        | Idempotently install docker-ce, the configured kind version, kubectl, and helm. |
| `setup-kind.sh`             | Create the kind cluster, label worker nodes for cache-server scheduling, and prepare `/var/lib/ssd/cacheserver` on every node container via `docker exec`. |
| `deploy-dacs.sh`            | Install the public DACS OCI chart from MCR; imports the cache-server and controller images directly into each kind node's containerd store. |
| `deploy-blobfuse2.sh`       | Render `docker/k8s/blobfuse2-dist-cache-deployment.yaml.tmpl` with storage credentials + image ref, side-load the blobfuse2 image into every kind node, apply, and wait for rollout. |
| `teardown-kind.sh`          | Delete the cluster. Best-effort (runs under `set +e`). |

## Local usage

```bash
# 1. Install prerequisites (one-time).
./test/scripts/dcache/install-prereqs.sh

# 2. (Optional) override cluster settings. Defaults live in
#    config/nightly.config. DACS artifacts are pinned to the public release:
#    chart 0.1.0-alpha.2, images v0.1.0-alpha.2.
# export CLUSTER_NAME=blobfuse-dcache
# export CACHE_SERVER_REPLICAS=3

# DACS artifacts are public: no Azure login or registry pull secret is needed.

# 3. Bring up the cluster and deploy DACS.
./test/scripts/dcache/setup-kind.sh
./test/scripts/dcache/deploy-dacs.sh

# 4. Build the in-cluster blobfuse2 image from the current tree, then deploy
#    the pod driven by test/dcache_e2e/. deploy-blobfuse2.sh does NOT pull;
#    the image must already exist in the local docker daemon.
(cd docker && ./buildcontainer.sh Dockerfile x86_64)
ver=$(./blobfuse2 --version | cut -d ' ' -f 3)
export BLOBFUSE2_IMAGE="azure-blobfuse2-x86_64.${ver}"
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
- `setup-kind.sh` labels workers with `dacs.azure.com/processing-unit=cpu`
  and prepares the chart's hostPath `/var/lib/ssd/cacheserver`.
  The NVMe/RAID `dacs-prereq` chart is not installed for kind.
- `deploy-dacs.sh` installs `oci://mcr.microsoft.com/dacs/helmchart/dacs`
  at version `0.1.0-alpha.2` with a three-server Cache resource named
  `cache-sample`. The controller and cache-server images use
  `mcr.microsoft.com/dacs/cache-controller:v0.1.0-alpha.2` and
  `mcr.microsoft.com/dacs/cache-server:v0.1.0-alpha.2`.
- Nothing here needs `MINIKUBE_HOME` or `/mnt/minikube`. kind stores node
  container state under Docker's data-root -- if `/` is tight on the agent,
  point Docker's data-root at `/mnt/docker` via `/etc/docker/daemon.json`.
