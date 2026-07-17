#!/bin/bash
#
# Bring up a local kind cluster shaped for Tachyon cache-server pods.
#
# Design notes:
#   * We generate the kind Cluster config on the fly from KIND_NODES so the
#     shape stays in one place (config/nightly.config). The static skeleton at
#     kind-cluster.yaml is retained as a reference / for hand-invocation.
#   * After the cluster is up we:
#       1. Label every node with the two `processing-unit` labels that the
#          vienna-tachyon Helm chart's nodeSelector expects.
#       2. Prepare /var/lib/ssd/cacheserver on every node container. The
#          Helm chart's `useHostPath: true` value points here and the pod
#          fails to schedule if the directory does not already exist.
#          `docker exec` is the kind equivalent of `minikube ssh -n <node>`.
#
# Keep the labeling + hostPath prep in sync with the assumptions in
# vienna-tachyon/helm/cache-server/values.yaml.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

CONFIG_FILE="$SCRIPT_DIR/config/nightly.config"
if [[ -f "$CONFIG_FILE" ]]; then
    # shellcheck source=./config/nightly.config
    source "$CONFIG_FILE"
    echo "Loaded config from: $CONFIG_FILE"
fi

CLUSTER_NAME="${CLUSTER_NAME:-blobfuse-dcache}"
KIND_NODES="${KIND_NODES:-4}"
KIND_NODE_IMAGE="${KIND_NODE_IMAGE:-kindest/node:v1.31.0}"

if [[ "$KIND_NODES" -lt 2 ]]; then
    echo "ERROR: KIND_NODES=$KIND_NODES; need at least 1 control-plane + 1 worker" >&2
    exit 1
fi

echo "Cluster config: $KIND_NODES nodes (1 control-plane + $((KIND_NODES - 1)) workers), image $KIND_NODE_IMAGE"

# --- Delete any pre-existing cluster with this name -----------------------
if kind get clusters 2>/dev/null | grep -qx "$CLUSTER_NAME"; then
    echo "Existing kind cluster '$CLUSTER_NAME' found. Deleting it..."
    kind delete cluster --name "$CLUSTER_NAME"
    echo "Cluster deleted successfully!"
fi

# --- Generate the kind cluster config -------------------------------------
GENERATED_CONFIG="$(mktemp -t kind-cluster.XXXXXX.yaml)"
trap 'rm -f "$GENERATED_CONFIG"' EXIT

{
    echo "kind: Cluster"
    echo "apiVersion: kind.x-k8s.io/v1alpha4"
    echo "name: $CLUSTER_NAME"
    echo "nodes:"
    echo "  - role: control-plane"
    for _ in $(seq 2 "$KIND_NODES"); do
        echo "  - role: worker"
    done
} > "$GENERATED_CONFIG"

echo "Generated kind cluster config:"
cat "$GENERATED_CONFIG"

# --- Create the cluster ---------------------------------------------------
echo ""
echo "Creating kind cluster '$CLUSTER_NAME'..."
kind create cluster \
    --name "$CLUSTER_NAME" \
    --image "$KIND_NODE_IMAGE" \
    --config "$GENERATED_CONFIG" \
    --wait 5m

echo "Cluster created successfully!"

# --- Wait for all nodes Ready ---------------------------------------------
echo "Waiting for all nodes to reach Ready..."
kubectl wait --for=condition=Ready node --all --timeout=120s

# --- Label nodes for cache-server scheduling ------------------------------
echo ""
echo "Labeling nodes for cache-server scheduling..."
for node in $(kubectl get nodes -o jsonpath='{.items[*].metadata.name}'); do
    kubectl label node "$node" singularity.azure.com/processing-unit=system --overwrite
    kubectl label node "$node" nexus.azure.com/processing-unit=cpu --overwrite
    echo "Labeled $node with system and cpu labels"
done

# --- Prepare /var/lib/ssd/cacheserver on every node -----------------------
# The Helm chart mounts /var/lib/ssd/cacheserver as a hostPath (useHostPath=true).
# In kind, "the node" is a docker container named `<cluster>-control-plane`,
# `<cluster>-worker`, `<cluster>-worker2`, ...
echo ""
echo "Preparing /var/lib/ssd/cacheserver on all kind node containers..."
for container in $(kind get nodes --name "$CLUSTER_NAME"); do
    docker exec "$container" mkdir -p /var/lib/ssd/cacheserver
    docker exec "$container" chmod 777 /var/lib/ssd/cacheserver
    docker exec "$container" ls -ld /var/lib/ssd/cacheserver
done

# --- Debug dump -----------------------------------------------------------
echo ""
echo "Node labels:"
kubectl get nodes --show-labels | grep -E "(nexus\.azure\.com/processing-unit|singularity\.azure\.com/processing-unit)"

echo ""
echo "Cluster setup complete!"
