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
#               DCACHE_DISCOVERY_ENDPOINT,
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
# The tachyon-cache operator names its discovery Service after the Cache CR
# (default CR name is 'cache-sample' when installed with cache.enabled=true).
# Override CACHE_CR_NAME if a non-default CR name is used.
CACHE_CR_NAME="${CACHE_CR_NAME:-cache-sample}"
DCACHE_DISCOVERY_ENDPOINT="${DCACHE_DISCOVERY_ENDPOINT:-${CACHE_CR_NAME}-discovery.${NAMESPACE:-cache-server}.svc.cluster.local:${CACHE_SERVER_PORT:-9065}}"
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
echo "Discovery endpoint         : $DCACHE_DISCOVERY_ENDPOINT"
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
    DCACHE_DISCOVERY_ENDPOINT STO_ACC_NAME STO_ACC_KEY STO_ACC_CONTAINER \
    STO_ACC_ENDPOINT

RENDERED="$(mktemp --suffix=.yaml)"

# Allow-list keeps future ${...} additions from being silently blanked.
envsubst '$BLOBFUSE2_IMAGE $BLOBFUSE2_NAMESPACE $BLOBFUSE2_DEPLOYMENT $DCACHE_DISCOVERY_ENDPOINT $STO_ACC_NAME $STO_ACC_KEY $STO_ACC_CONTAINER $STO_ACC_ENDPOINT' \
    < "$MANIFEST_TEMPLATE" > "$RENDERED"

echo "Applying rendered manifest ..."
kubectl apply -f "$RENDERED"

echo "Waiting for rollout of deployment/$BLOBFUSE2_DEPLOYMENT in namespace $BLOBFUSE2_NAMESPACE ..."
if ! kubectl -n "$BLOBFUSE2_NAMESPACE" rollout status \
        "deployment/$BLOBFUSE2_DEPLOYMENT" --timeout=5m; then
    echo ""
    echo "=========================================="
    echo "Rollout FAILED - diagnostics"
    echo "=========================================="

    echo ""
    echo "--- deployment ---"
    kubectl -n "$BLOBFUSE2_NAMESPACE" get deploy "$BLOBFUSE2_DEPLOYMENT" -o wide || true
    kubectl -n "$BLOBFUSE2_NAMESPACE" describe deploy "$BLOBFUSE2_DEPLOYMENT" || true

    echo ""
    echo "--- pods ---"
    kubectl -n "$BLOBFUSE2_NAMESPACE" get pods -o wide --show-labels || true
    for pod in $(kubectl -n "$BLOBFUSE2_NAMESPACE" get pods \
                 -l "app=$BLOBFUSE2_DEPLOYMENT" \
                 -o jsonpath='{.items[*].metadata.name}' 2>/dev/null); do
        echo ""
        echo "--- describe pod/$pod ---"
        kubectl -n "$BLOBFUSE2_NAMESPACE" describe pod "$pod" || true
        echo ""
        echo "--- logs pod/$pod (all containers, current) ---"
        kubectl -n "$BLOBFUSE2_NAMESPACE" logs "$pod" --all-containers=true --tail=200 || true
        echo ""
        echo "--- logs pod/$pod (all containers, previous) ---"
        kubectl -n "$BLOBFUSE2_NAMESPACE" logs "$pod" --all-containers=true --tail=200 --previous || true
    done

    echo ""
    echo "--- events ---"
    kubectl -n "$BLOBFUSE2_NAMESPACE" get events --sort-by=.lastTimestamp || true

    echo ""
    echo "--- discovery endpoint sanity (does the Service exist?) ---"
    # DCACHE_DISCOVERY_ENDPOINT is host:port; strip port then FQDN's namespace piece.
    ep_host="${DCACHE_DISCOVERY_ENDPOINT%:*}"
    svc_name="${ep_host%%.*}"
    svc_ns="${NAMESPACE:-cache-server}"
    echo "expected Service: $svc_name in namespace $svc_ns"
    kubectl -n "$svc_ns" get svc "$svc_name" -o wide || true
    kubectl -n "$svc_ns" get endpoints "$svc_name" -o wide || true

    echo ""
    echo "--- cache-server headless Service + pods (peers blobfuse2 will dial) ---"
    kubectl -n "$svc_ns" get svc,pods -o wide -l app=cacheserver || true

    echo ""
    echo "--- DNS probe from a debug pod in $BLOBFUSE2_NAMESPACE ---"
    kubectl -n "$BLOBFUSE2_NAMESPACE" run dcache-dnsprobe \
        --rm -i --restart=Never --image=busybox:1.36 --timeout=60s -- \
        sh -c "
          set +e
          echo '# nslookup discovery endpoint'
          nslookup $ep_host
          echo '# nslookup headless service ${CACHE_CR_NAME}.$svc_ns.svc.cluster.local'
          nslookup ${CACHE_CR_NAME}.$svc_ns.svc.cluster.local
          for i in 0 1 2; do
            host=${CACHE_CR_NAME}-\$i.${CACHE_CR_NAME}.$svc_ns.svc.cluster.local
            echo \"# nslookup peer \$host\"
            nslookup \$host
          done
        " || true

    exit 1
fi

echo ""
echo "=========================================="
echo "Manual blobfuse2 mount diagnostic"
echo "=========================================="

POD_NAME="$(kubectl -n "$BLOBFUSE2_NAMESPACE" get pods \
    -l "app=$BLOBFUSE2_DEPLOYMENT" \
    -o jsonpath='{.items[0].metadata.name}')"

if [[ -z "$POD_NAME" ]]; then
    echo "ERROR: no blobfuse2 pod found for deployment/$BLOBFUSE2_DEPLOYMENT" >&2
    exit 1
fi

echo "Pod: $POD_NAME"
echo ""
echo "--- binary identity + linkage ---"
kubectl -n "$BLOBFUSE2_NAMESPACE" exec "$POD_NAME" -- \
    sh -c '
        sha256sum /usr/local/bin/blobfuse2
        ldd /usr/local/bin/blobfuse2
        echo "# embedded top-level error marker"
        grep -a -F -m1 "blobfuse2: " /usr/local/bin/blobfuse2 >/dev/null &&
            echo "present" || echo "missing"
    '

echo ""
echo "--- basic CLI execution inside kind pod ---"
set +e
kubectl -n "$BLOBFUSE2_NAMESPACE" exec "$POD_NAME" -- \
    timeout 15s /usr/local/bin/blobfuse2 --version
VERSION_RC=$?
echo "blobfuse2 --version exit code: $VERSION_RC"

kubectl -n "$BLOBFUSE2_NAMESPACE" exec "$POD_NAME" -- \
    timeout 15s /usr/local/bin/blobfuse2 --help
HELP_RC=$?
echo "blobfuse2 --help exit code: $HELP_RC"

echo ""
echo "--- Go package initialization trace inside kind pod ---"
kubectl -n "$BLOBFUSE2_NAMESPACE" exec "$POD_NAME" -- \
    sh -c 'GODEBUG=inittrace=1 timeout 15s /usr/local/bin/blobfuse2 --version'
INITTRACE_RC=$?
echo "blobfuse2 inittrace exit code: $INITTRACE_RC"
set -e

echo ""
echo "--- mount directory ---"
kubectl -n "$BLOBFUSE2_NAMESPACE" exec "$POD_NAME" -- \
    sh -c 'ls -ld /mnt/blobfuse_mnt; find /mnt/blobfuse_mnt -mindepth 1 -maxdepth 1 -printf "%f\n"'

echo ""
echo "--- rendered config (account key redacted) ---"
kubectl -n "$BLOBFUSE2_NAMESPACE" exec "$POD_NAME" -- \
    sh -c 'sed -E "s/^([[:space:]]*account-key:).*/\1 \"<redacted>\"/" /usr/share/blobfuse2/config.yaml'

echo ""
echo "--- executing blobfuse2 mount (90s diagnostic timeout) ---"
set +e
kubectl -n "$BLOBFUSE2_NAMESPACE" exec "$POD_NAME" -- \
    timeout 90s blobfuse2 mount /mnt/blobfuse_mnt \
        --config-file=/usr/share/blobfuse2/config.yaml \
        --ignore-open-flags \
        --foreground=true \
        --disable-version-check=true
MOUNT_RC=$?
set -e

echo ""
if [[ "$MOUNT_RC" -eq 124 ]]; then
    echo "DIAGNOSTIC RESULT: blobfuse2 remained running for 90s and was stopped by timeout."
    echo "The immediate exit does not reproduce when invoked manually in the same kind pod."
else
    echo "DIAGNOSTIC RESULT: blobfuse2 exited with code $MOUNT_RC."
    echo "The command output above is the direct startup failure from inside the kind pod."
fi

# This temporary diagnostic deployment intentionally runs sleep as PID 1, so
# cloned E2E Deployments would not mount blobfuse2. Stop here after collecting
# the direct result instead of allowing misleading test failures.
exit 1
