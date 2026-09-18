# distributed_cache nightly E2E helper scripts

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
| `docker/Dockerfile.dcache-runtime` | Build blobfuse2 and the test fault server inside the selected distro, then create the runtime image used by the suite. |
| `teardown-kind.sh`          | Delete the cluster. Best-effort (runs under `set +e`). |

## Local usage

Concurrency tests default to 5 pods, 2 readers per pod, and 3 files. Override
the bounded workload size through test arguments:

```bash
go test -v -tags=fuse3 ./test/dcache_e2e -run '^TestConcurrency_' -args \
  -concurrency-pods=5 \
  -concurrency-readers-per-pod=2 \
  -concurrency-files=3
```

```bash
# 1. Install prerequisites (one-time).
./test/scripts/dcache/install-prereqs.sh

# 2. (Optional) override Tachyon image + chart coordinates. Defaults live in
#    config/nightly.config (registry + repo pre-populated, tags left blank);
#    deploy-tachyon.sh auto-resolves the newest image tag AND chart version
#    via `az acr repository show-tags` when the respective env var is unset.
#    Override any of these to pin.
# export CACHE_SERVER_IMAGE_REGISTRY=<acr-name>.azurecr.io
# export CACHE_SERVER_IMAGE_REPO=cache-server
# export CACHE_SERVER_IMAGE_TAG=<image-tag>
# export CACHE_SERVER_CHART_REGISTRY=<other-acr>.azurecr.io
# export CACHE_SERVER_CHART_REPO=charts/cache-server
# export CACHE_SERVER_PREREQ_CHART_REPO=charts/cache-server-prereq
# export CACHE_SERVER_CHART_VERSION=<chart-version>

# ACR access: deploy-tachyon.sh calls `az acr login --expose-token` to mint
# an in-cluster pull secret, so `az login` must already have succeeded with
# an identity that has AcrPull on the registry.

# 3. Bring up the cluster and deploy Tachyon.
./test/scripts/dcache/setup-kind.sh
./test/scripts/dcache/deploy-tachyon.sh

# 4. Build the in-cluster blobfuse2 image from the current tree, then deploy
#    the pod driven by test/dcache_e2e/. deploy-blobfuse2.sh does NOT pull;
#    the image must already exist in the local docker daemon.
docker build \
  --build-arg BASE_IMAGE=mcr.microsoft.com/mirror/docker/library/ubuntu:22.04 \
  --build-arg PACKAGE_FAMILY=deb \
  -t azure-blobfuse2-dcache-ubuntu:local \
  -f docker/Dockerfile.dcache-runtime .
export BLOBFUSE2_IMAGE=azure-blobfuse2-dcache-ubuntu:local
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
