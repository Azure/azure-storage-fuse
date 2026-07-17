# dist_cache nightly E2E helper scripts

These scripts stand up a local [kind](https://kind.sigs.k8s.io/) cluster,
deploy the [Tachyon](https://github.com/Azure/Tachyon) cache-server via its
Helm chart, and expose the cacheserver pods on localhost so blobfuse2 running
on the same host can point `dist_cache.server-list` at them.

They are invoked from `azure-pipeline-templates/dist-cache-e2e.yml` in the
nightly build, but are self-contained enough to run locally for iterative
debugging.

See [PLAN.md](PLAN.md) for the design rationale (in particular §1 on why we
chose kind over minikube even though the upstream vienna-tachyon tooling
targets minikube).

## Files

| File | Purpose |
|---|---|
| `config/nightly.config`     | Shared bash config: cluster shape, image coordinates, namespace. Every value is overridable via env var. |
| `install-prereqs.sh`        | Idempotently install docker-ce, kind, kubectl, helm, netcat. |
| `setup-kind.sh`             | Create the kind cluster, label nodes for cache-server scheduling, prepare `/var/lib/ssd/cacheserver` on every node container via `docker exec`. |
| `kind-cluster.yaml`         | Reference kind `Cluster` config (setup-kind.sh regenerates its own from `KIND_NODES` at runtime). |
| `deploy-tachyon.sh`         | Install the `cache-server-prereq` chart, then the `cache-server` chart, both pulled directly from an OCI-enabled ACR (`oci://...`); side-loads the image with `kind load docker-image`. |
| `expose-cacheserver.sh`     | Start `kubectl port-forward` for each cacheserver pod on sequential localhost ports and emit a comma-separated server list. |
| `teardown-kind.sh`          | Kill port-forwards, uninstall the helm release, and delete the cluster. Best-effort (runs under `set +e`). |

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

# 4. Expose cacheserver pods on localhost:9065, 9066, 9067, ...
export DCACHE_SERVER_LIST_FILE=/tmp/dcache_server_list.txt
export DCACHE_PORTFORWARD_PIDS_FILE=/tmp/dcache_portforward_pids.txt
./test/scripts/dcache/expose-cacheserver.sh
export DCACHE_SERVERS=$(cat "$DCACHE_SERVER_LIST_FILE")

# 5. Generate a blobfuse2 config and mount.
./blobfuse2 gen-test-config \
    --config-file=testdata/config/azure_key_dist_cache_block_e2e.yaml \
    --container-name=<container> \
    --temp-path=/tmp/blobfuse2_tmp \
    --output-file=/tmp/blobfuse2_dcache.yaml
./blobfuse2 mount /tmp/mnt --config-file=/tmp/blobfuse2_dcache.yaml

# 6. Teardown.
./test/scripts/dcache/teardown-kind.sh
```

## Notes

- The kind cluster shape (4 nodes: 1 control-plane + 3 workers) is set by
  `KIND_NODES` in `config/nightly.config`. `setup-kind.sh` generates the
  `kind create cluster --config=...` file on the fly from that value.
- The kind node image is pinned by `KIND_NODE_IMAGE` (default
  `kindest/node:v1.31.0`); bump when you want a newer Kubernetes.
- `deploy-tachyon.sh` sets `cacheServer.scheduler.enabled=false` because
  blobfuse2 E2E tests do not need the scheduler component.
- `expose-cacheserver.sh` runs `kubectl port-forward` in the background and
  polls the local port with `nc -z` before returning, so callers can assume the
  ports are actually listening once the script exits.
- Nothing here needs `MINIKUBE_HOME` or `/mnt/minikube`. kind stores node
  container state under Docker's data-root -- if `/` is tight on the agent,
  point Docker's data-root at `/mnt/docker` via `/etc/docker/daemon.json`.
