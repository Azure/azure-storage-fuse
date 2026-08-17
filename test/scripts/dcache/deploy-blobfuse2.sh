#!/bin/bash
#
# Deploy the in-cluster blobfuse2 pod that test/dcache_e2e/... drives.
# Renders docker/k8s/blobfuse2-dist-cache-deployment.yaml.tmpl via envsubst,
# side-loads the image into every kind node, applies, waits for rollout.
# Shared entry point for the ADO pipeline and local runs.
#
# The image referenced by BLOBFUSE2_IMAGE must already be present in the
# local docker daemon (built via docker/buildcontainer.sh); this script does
# not pull it from any registry.
#
# Required env: BLOBFUSE2_IMAGE, STO_ACC_NAME, STO_ACC_KEY,
#               STO_ACC_CONTAINER (or $containerName from the pipeline).
# Optional env: BLOBFUSE2_NAMESPACE, BLOBFUSE2_DEPLOYMENT, STO_ACC_ENDPOINT,
#               DCACHE_DISCOVERY_URL,
#               BLOBFUSE2_IMAGE_LOAD (default true; set false for non-kind),
#               MANIFEST_TEMPLATE.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

# Load shared configuration (CLUSTER_NAME, NAMESPACE, CACHE_SERVER_PORT, ...)
CONFIG_FILE="$SCRIPT_DIR/config/nightly.config"
if [[ -f "$CONFIG_FILE" ]]; then
    # shellcheck source=./config/nightly.config
    source "$CONFIG_FILE"
    echo "Loaded config from: $CONFIG_FILE"
fi

# --- Required inputs -------------------------------------------------------

: "${BLOBFUSE2_IMAGE:?ERROR: BLOBFUSE2_IMAGE is empty (expected full ref, e.g. myacr.azurecr.io/azure-blobfuse2:2.5.4)}"
: "${STO_ACC_NAME:?ERROR: STO_ACC_NAME is empty}"
: "${STO_ACC_KEY:?ERROR: STO_ACC_KEY is empty}"

# The pipeline exports the container name as `containerName`; accept either.
STO_ACC_CONTAINER="${STO_ACC_CONTAINER:-${containerName:-}}"
if [[ -z "$STO_ACC_CONTAINER" ]]; then
    echo "ERROR: STO_ACC_CONTAINER (or containerName) is empty." >&2
    exit 1
fi

# --- Derived defaults ------------------------------------------------------

BLOBFUSE2_NAMESPACE="${BLOBFUSE2_NAMESPACE:-blobfuse2-dist-cache}"
BLOBFUSE2_DEPLOYMENT="${BLOBFUSE2_DEPLOYMENT:-blobfuse2-dist-cache}"
STO_ACC_ENDPOINT="${STO_ACC_ENDPOINT:-https://${STO_ACC_NAME}.blob.core.windows.net}"
DCACHE_DISCOVERY_URL="${DCACHE_DISCOVERY_URL:-cacheserver-discovery.${NAMESPACE:-cache-server}.svc.cluster.local:${CACHE_SERVER_PORT:-9065}}"
BLOBFUSE2_IMAGE_LOAD="${BLOBFUSE2_IMAGE_LOAD:-true}"
MANIFEST_TEMPLATE="${MANIFEST_TEMPLATE:-$REPO_ROOT/docker/k8s/blobfuse2-dist-cache-deployment.yaml.tmpl}"

if [[ ! -f "$MANIFEST_TEMPLATE" ]]; then
    echo "ERROR: manifest template not found: $MANIFEST_TEMPLATE" >&2
    exit 1
fi

# STO_ACC_KEY is intentionally not echoed.
echo "Using blobfuse2 image      : $BLOBFUSE2_IMAGE"
echo "Namespace                  : $BLOBFUSE2_NAMESPACE"
echo "Deployment                 : $BLOBFUSE2_DEPLOYMENT"
echo "Discovery URL              : $DCACHE_DISCOVERY_URL"
echo "Storage account            : $STO_ACC_NAME"
echo "Storage container          : $STO_ACC_CONTAINER"
echo "Storage endpoint           : $STO_ACC_ENDPOINT"
echo "Manifest template          : $MANIFEST_TEMPLATE"
echo "Side-load into kind nodes  : $BLOBFUSE2_IMAGE_LOAD"

# --- Validate the image is present locally --------------------------------

if ! docker image inspect "$BLOBFUSE2_IMAGE" >/dev/null 2>&1; then
    echo "ERROR: image '$BLOBFUSE2_IMAGE' not found in the local docker daemon." >&2
    echo "       Build it first via docker/buildcontainer.sh (see test/scripts/dcache/README.md)." >&2
    exit 1
fi

IMAGE_TAR=""
RENDERED=""
cleanup() {
    [[ -n "$IMAGE_TAR" ]] && rm -f "$IMAGE_TAR"
    [[ -n "$RENDERED" ]] && rm -f "$RENDERED"
}
trap cleanup EXIT

# --- Side-load image into every kind node ---------------------------------
# See deploy-tachyon.sh for why we use `ctr images import` instead of `kind load`.

if [[ "$BLOBFUSE2_IMAGE_LOAD" == "true" ]]; then
    if ! command -v kind >/dev/null 2>&1; then
        echo "ERROR: kind CLI not found (needed to side-load image; set BLOBFUSE2_IMAGE_LOAD=false to skip)." >&2
        exit 1
    fi
    if ! kind get clusters | grep -qx "${CLUSTER_NAME:-blobfuse-dcache}"; then
        echo "ERROR: kind cluster '${CLUSTER_NAME:-blobfuse-dcache}' not found. Run setup-kind.sh first" >&2
        echo "       (or set BLOBFUSE2_IMAGE_LOAD=false if targeting a non-kind cluster)." >&2
        exit 1
    fi

    IMAGE_TAR="$(mktemp --suffix=.tar)"
    echo "Saving image to $IMAGE_TAR ..."
    docker save "$BLOBFUSE2_IMAGE" -o "$IMAGE_TAR"

    echo "Importing image into every node of kind cluster '$CLUSTER_NAME' ..."
    for node in $(kind get nodes --name "$CLUSTER_NAME"); do
        echo "  -> $node"
        docker exec -i "$node" ctr --namespace=k8s.io images import \
            --digests --snapshotter=overlayfs - < "$IMAGE_TAR"
    done
fi

# --- Render + apply manifest -----------------------------------------------

if ! command -v envsubst >/dev/null 2>&1; then
    echo "ERROR: envsubst not found (install the gettext package)." >&2
    exit 1
fi

export BLOBFUSE2_IMAGE BLOBFUSE2_NAMESPACE BLOBFUSE2_DEPLOYMENT \
    DCACHE_DISCOVERY_URL STO_ACC_NAME STO_ACC_KEY STO_ACC_CONTAINER \
    STO_ACC_ENDPOINT

RENDERED="$(mktemp --suffix=.yaml)"

# Allow-list keeps future ${...} additions from being silently blanked.
envsubst '$BLOBFUSE2_IMAGE $BLOBFUSE2_NAMESPACE $BLOBFUSE2_DEPLOYMENT $DCACHE_DISCOVERY_URL $STO_ACC_NAME $STO_ACC_KEY $STO_ACC_CONTAINER $STO_ACC_ENDPOINT' \
    < "$MANIFEST_TEMPLATE" > "$RENDERED"

echo "Applying rendered manifest ..."
kubectl apply -f "$RENDERED"

echo "Waiting for rollout of deployment/$BLOBFUSE2_DEPLOYMENT in namespace $BLOBFUSE2_NAMESPACE ..."
kubectl -n "$BLOBFUSE2_NAMESPACE" rollout status \
    "deployment/$BLOBFUSE2_DEPLOYMENT" --timeout=5m

echo "blobfuse2 pod deployed."
