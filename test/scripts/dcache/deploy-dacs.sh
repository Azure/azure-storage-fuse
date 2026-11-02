#!/bin/bash
#
# Deploy the public DACS Helm chart into the local kind cluster.
# The chart bundles the manager (control plane) and cache-server (data plane).
#
# Import images directly with ctr rather than kind load: Docker's containerd
# image store can save a multi-platform manifest with only the pulled platform's
# blobs, which fails with kind load's --all-platforms import.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

CONFIG_FILE="$SCRIPT_DIR/config/nightly.config"
if [[ -f "$CONFIG_FILE" ]]; then
    # shellcheck source=./config/nightly.config
    source "$CONFIG_FILE"
    echo "Loaded config from: $CONFIG_FILE"
fi

# Side-load the images used by the pinned chart's defaults.
CACHE_SERVER_IMAGE="mcr.microsoft.com/dacs/cache-server:v${DACS_CHART_VERSION}"
CACHE_CONTROLLER_IMAGE="mcr.microsoft.com/dacs/cache-controller:v${DACS_CHART_VERSION}"

echo "Using cache-server image : $CACHE_SERVER_IMAGE"
echo "Using controller image   : $CACHE_CONTROLLER_IMAGE"
echo "Using chart              : $DACS_CHART_REF (version $DACS_CHART_VERSION)"
echo "Namespace                : $NAMESPACE"
echo "Release                  : $RELEASE_NAME"
echo "Replicas                 : $CACHE_SERVER_REPLICAS"

# --- Pull + side-load the images ------------------------------------------

docker pull "$CACHE_SERVER_IMAGE"
docker pull "$CACHE_CONTROLLER_IMAGE"

IMAGE_TAR="$(mktemp --suffix=.tar)"
CONTROLLER_TAR="$(mktemp --suffix=.tar)"
trap 'rm -f "$IMAGE_TAR" "$CONTROLLER_TAR"' EXIT

echo "Saving cache-server image to $IMAGE_TAR ..."
docker save "$CACHE_SERVER_IMAGE" -o "$IMAGE_TAR"
echo "Saving controller image to $CONTROLLER_TAR ..."
docker save "$CACHE_CONTROLLER_IMAGE" -o "$CONTROLLER_TAR"

echo "Importing images into every node of kind cluster '$CLUSTER_NAME'..."
for node in $(kind get nodes --name "$CLUSTER_NAME"); do
    echo "  -> $node (cache-server)"
    # Stream on stdin: docker cp writes beneath the kind node's /tmp tmpfs,
    # making the copied archive invisible to ctr inside the node.
    docker exec -i "$node" ctr --namespace=k8s.io images import \
        --digests --snapshotter=overlayfs - < "$IMAGE_TAR"
    echo "  -> $node (controller)"
    docker exec -i "$node" ctr --namespace=k8s.io images import \
        --digests --snapshotter=overlayfs - < "$CONTROLLER_TAR"
done

# --- Deploy via Helm (OCI) -------------------------------------------------

kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

helm uninstall "$RELEASE_NAME" -n "$NAMESPACE" --ignore-not-found

echo "Deploying DACS Helm chart from $DACS_CHART_REF ..."
helm install "$RELEASE_NAME" "$DACS_CHART_REF" \
    --version "$DACS_CHART_VERSION" \
    -n "$NAMESPACE" \
    --set cache.enabled=true \
    --set cache.scaling.static.numServers="$CACHE_SERVER_REPLICAS"

# --- Wait for pods to come up ---------------------------------------------

CACHE_SERVER_READY_TIMEOUT="${CACHE_SERVER_READY_TIMEOUT:-600}"
echo "Waiting up to ${CACHE_SERVER_READY_TIMEOUT}s for cacheserver replicas to become ready..."
deadline=$(( $(date +%s) + CACHE_SERVER_READY_TIMEOUT ))
while true; do
    ready_pods=$(kubectl get pods -n "$NAMESPACE" -l app=cacheserver --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l)
    total_pods=$(kubectl get pods -n "$NAMESPACE" -l app=cacheserver --no-headers 2>/dev/null | wc -l)
    # Reinstalling the chart can leave terminating pods from the old Cache.
    ready_replicas=$(kubectl get statefulset cache-sample -n "$NAMESPACE" \
        --ignore-not-found -o jsonpath='{.status.readyReplicas}')
    if [[ "$ready_pods" -eq "$total_pods" && "$total_pods" -gt 0 && \
          "${ready_replicas:-0}" -eq "$CACHE_SERVER_REPLICAS" ]]; then
        echo "All $ready_replicas cacheserver replicas are ready."
        break
    fi
    if [[ $(date +%s) -ge $deadline ]]; then
        echo "ERROR: cacheserver replicas did not become ready within ${CACHE_SERVER_READY_TIMEOUT}s" \
             "($ready_pods / $total_pods)." >&2
        echo "--- pods in namespace '$NAMESPACE' ---" >&2
        kubectl get pods -n "$NAMESPACE" -o wide >&2 || true
        echo "--- describe non-Running pods ---" >&2
        for pod in $(kubectl get pods -n "$NAMESPACE" -l app=cacheserver \
                     --field-selector=status.phase!=Running \
                     -o jsonpath='{.items[*].metadata.name}' 2>/dev/null); do
            kubectl describe pod -n "$NAMESPACE" "$pod" >&2 || true
        done
        exit 1
    fi
    echo "  $ready_pods / $total_pods pods Running; sleeping 5s..."
    sleep 5
done

# --- Debug dump ------------------------------------------------------------

echo ""
echo "=========================================="
echo "CacheServer Deployment Status"
echo "=========================================="
echo ""

echo "StatefulSet:"
kubectl get statefulset -n "$NAMESPACE" || true
echo ""

echo "Services:"
kubectl get svc -n "$NAMESPACE" || true
echo ""

echo "Pods:"
kubectl get pods -n "$NAMESPACE" -o wide || true
echo ""

echo "Cacheserver pod DNS names (useful for debugging server-list):"
# The controller names the governing Service after the Cache CR, not the
# app=cacheserver label.
for pod in $(kubectl get pods -n "$NAMESPACE" -l app=cacheserver -o jsonpath='{.items[*].metadata.name}'); do
    echo "  $pod.cache-sample.$NAMESPACE.svc.cluster.local:$CACHE_SERVER_PORT"
done

echo ""
echo "Cache server deployed successfully!"
