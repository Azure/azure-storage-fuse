#!/bin/bash
#
# Deploy the Tachyon cache-server Helm chart into the local kind cluster.
#
# One chart is installed, pulled from an OCI-enabled ACR:
#   tachyon-cache - manager (control plane) + cache-server (data plane).
#
# Overrides we set on top of the chart's baked-in values.yaml:
#   * manager.image.repository / .tag             - controller image in ACR
#   * manager.cacheServerImage.repository / .tag  - data plane image in ACR
#
# Substrate note: upstream uses `minikube image load`; we `docker save` the
# image and drive `ctr images import` on each kind node directly. We do NOT
# use `kind load` (either variant) because it hardcodes
# `ctr images import --all-platforms`, which fails with
# `ctr: content digest ...: not found` when the archive from Docker's
# containerd image store references platforms whose blobs weren't pulled.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load shared configuration
CONFIG_FILE="$SCRIPT_DIR/config/nightly.config"
if [[ -f "$CONFIG_FILE" ]]; then
    # shellcheck source=./config/nightly.config
    source "$CONFIG_FILE"
    echo "Loaded config from: $CONFIG_FILE"
fi

# --- Required inputs -------------------------------------------------------

if [[ -z "${CACHE_SERVER_IMAGE_REGISTRY:-}" ]]; then
    echo "ERROR: CACHE_SERVER_IMAGE_REGISTRY is empty." >&2
    echo "       Set CACHE_SERVER_IMAGE_REGISTRY / _REPO (typically via the" >&2
    echo "       NightlyBlobFuse pipeline variable group) before running." >&2
    exit 1
fi

# Image tag auto-resolution: when unset, pick the most recently pushed tag
# under $CACHE_SERVER_IMAGE_REPO in the image ACR. Symmetrical with the chart
# resolver below so callers can leave both blank to always get the newest
# published bits.
if [[ -z "${CACHE_SERVER_IMAGE_TAG:-}" ]]; then
    if ! command -v az >/dev/null 2>&1; then
        echo "ERROR: CACHE_SERVER_IMAGE_TAG is empty and az CLI not found." >&2
        echo "       Either set CACHE_SERVER_IMAGE_TAG or install az + run 'az login'." >&2
        exit 1
    fi
    IMAGE_ACR_NAME="${CACHE_SERVER_IMAGE_REGISTRY%%.*}"
    echo "Resolving latest image tag from ACR '$IMAGE_ACR_NAME' repo '$CACHE_SERVER_IMAGE_REPO' ..."
    CACHE_SERVER_IMAGE_TAG="$(
        az acr repository show-tags \
            --name "$IMAGE_ACR_NAME" \
            --repository "$CACHE_SERVER_IMAGE_REPO" \
            --orderby time_desc \
            --top 1 \
            --output tsv 2>/dev/null | head -n1 | tr -d '\r' || true
    )"
    if [[ -z "$CACHE_SERVER_IMAGE_TAG" ]]; then
        echo "ERROR: could not resolve latest image tag from ACR." >&2
        echo "       Confirm 'az acr repository show-tags --name $IMAGE_ACR_NAME --repository $CACHE_SERVER_IMAGE_REPO' works." >&2
        exit 1
    fi
    echo "Resolved CACHE_SERVER_IMAGE_TAG=$CACHE_SERVER_IMAGE_TAG"
fi

CACHE_SERVER_IMAGE="${CACHE_SERVER_IMAGE_REGISTRY}/${CACHE_SERVER_IMAGE_REPO}:${CACHE_SERVER_IMAGE_TAG}"
# Controller image lives in the same ACR and uses the same tag as the
# cache-server image (that's how Tachyon builds are cut). Override
# CACHE_CONTROLLER_IMAGE_TAG only if a mismatched build needs to be tested.
CACHE_CONTROLLER_IMAGE_REPO="${CACHE_CONTROLLER_IMAGE_REPO:-cache-controller}"
CACHE_CONTROLLER_IMAGE_TAG="${CACHE_CONTROLLER_IMAGE_TAG:-$CACHE_SERVER_IMAGE_TAG}"
CACHE_CONTROLLER_IMAGE="${CACHE_SERVER_IMAGE_REGISTRY}/${CACHE_CONTROLLER_IMAGE_REPO}:${CACHE_CONTROLLER_IMAGE_TAG}"
CACHE_SERVER_CHART_REF="oci://${CACHE_SERVER_CHART_REGISTRY}/${CACHE_SERVER_CHART_REPO}"

# Chart version auto-resolution: when unset, pick the most recently pushed tag
# under $CACHE_SERVER_CHART_REPO in the chart ACR.
if [[ -z "${CACHE_SERVER_CHART_VERSION:-}" ]]; then
    if ! command -v az >/dev/null 2>&1; then
        echo "ERROR: CACHE_SERVER_CHART_VERSION is empty and az CLI not found." >&2
        echo "       Either set CACHE_SERVER_CHART_VERSION or install az + run 'az login'." >&2
        exit 1
    fi
    CHART_ACR_NAME="${CACHE_SERVER_CHART_REGISTRY%%.*}"
    echo "Resolving latest chart tag from ACR '$CHART_ACR_NAME' repo '$CACHE_SERVER_CHART_REPO' ..."
    CACHE_SERVER_CHART_VERSION="$(
        az acr repository show-tags \
            --name "$CHART_ACR_NAME" \
            --repository "$CACHE_SERVER_CHART_REPO" \
            --orderby time_desc \
            --top 1 \
            --output tsv 2>/dev/null | head -n1 | tr -d '\r' || true
    )"
    if [[ -z "$CACHE_SERVER_CHART_VERSION" ]]; then
        echo "ERROR: could not resolve latest chart version from ACR." >&2
        echo "       Confirm 'az acr repository show-tags --name $CHART_ACR_NAME --repository $CACHE_SERVER_CHART_REPO' works." >&2
        exit 1
    fi
    echo "Resolved CACHE_SERVER_CHART_VERSION=$CACHE_SERVER_CHART_VERSION"
fi

echo "Using cache-server image : $CACHE_SERVER_IMAGE"
echo "Using controller image   : $CACHE_CONTROLLER_IMAGE"
echo "Using chart              : $CACHE_SERVER_CHART_REF (version $CACHE_SERVER_CHART_VERSION)"
echo "Namespace                : $NAMESPACE"
echo "Release                  : $RELEASE_NAME"
echo "Replicas                 : $CACHE_SERVER_REPLICAS"

# --- Registry auth (host docker) -------------------------------------------
#
# `az login` alone does not populate ~/.docker/config.json; `az acr login`
# does, and Helm 3.10+ falls back to that file for OCI creds. The second
# call uses --expose-token to get the raw AAD token for the in-cluster
# pull secret minted below.

if ! command -v az >/dev/null 2>&1; then
    echo "ERROR: az CLI not found; cannot authenticate to ACR." >&2
    echo "       Install az and run 'az login' before invoking this script." >&2
    exit 1
fi

ACR_NAME="${CACHE_SERVER_IMAGE_REGISTRY%%.*}"

echo "Logging docker into ACR '$ACR_NAME' ..."
az acr login --name "$ACR_NAME" >/dev/null

echo "Fetching ACR access token for registry '$ACR_NAME' ..."
ACR_TOKEN="$(az acr login --name "$ACR_NAME" --expose-token --output tsv --query accessToken)"
if [[ -z "$ACR_TOKEN" ]]; then
    echo "ERROR: failed to obtain ACR access token for '$ACR_NAME'." >&2
    echo "       Run 'az login' and confirm you have AcrPull on the registry." >&2
    exit 1
fi

# --- Pull + side-load the images ------------------------------------------

echo "Pulling images ..."
docker pull "$CACHE_SERVER_IMAGE"
docker pull "$CACHE_CONTROLLER_IMAGE"

# Side-load images into every kind node manually rather than using
# `kind load docker-image` or `kind load image-archive`.
#
# `kind load` invokes `ctr images import --all-platforms` inside the node,
# which requires the archive to contain blobs for EVERY platform referenced
# in the manifest list. When Docker is using the containerd image store (the
# default on newer Docker versions), `docker save` emits the full manifest
# list but only the blobs for the single platform it pulled -- so kind fails
# with:
#     ctr: content digest sha256:...: not found
# Driving `ctr images import` ourselves (without `--all-platforms`) makes
# containerd import only the platforms actually present in the archive,
# which is what we want.
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
    # Stream the archive on stdin instead of `docker cp`-ing it into the node
    # first: kind nodes have a tmpfs mount over /tmp that hides files written
    # via `docker cp` (which targets the underlying overlay layer), so the
    # copy silently succeeds but `ctr` inside the node sees no such file.
    docker exec -i "$node" ctr --namespace=k8s.io images import \
        --digests --snapshotter=overlayfs - < "$IMAGE_TAR"
    echo "  -> $node (controller)"
    docker exec -i "$node" ctr --namespace=k8s.io images import \
        --digests --snapshotter=overlayfs - < "$CONTROLLER_TAR"
done

# --- Deploy via Helm (OCI) -------------------------------------------------

kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

# --- ACR pull secret ------------------------------------------------------
#
# The kind nodes are separate containers with their own containerd runtime;
# they do NOT inherit the host's `az acr login` docker credentials. Any pod
# whose image is NOT the one we side-loaded above (e.g. cacheserver-scheduler
# pulled by the prereq chart) will fail to pull from ACR with a 401.
#
# We mint a short-lived AAD access token via `az acr login --expose-token`
# and drop it into a Kubernetes docker-registry secret in the target
# namespace. Special username 00000000-0000-0000-0000-000000000000 tells
# ACR that the password is an AAD access token.
#
# The secret is attached to the `default` ServiceAccount up front and to
# every ServiceAccount the charts create, right after each helm install
# (see `attach_pull_secret_to_all_sas` below).
ACR_PULL_SECRET_NAME="${ACR_PULL_SECRET_NAME:-acr-pull}"

kubectl delete secret "$ACR_PULL_SECRET_NAME" -n "$NAMESPACE" --ignore-not-found
kubectl create secret docker-registry "$ACR_PULL_SECRET_NAME" \
    --docker-server="$CACHE_SERVER_IMAGE_REGISTRY" \
    --docker-username="00000000-0000-0000-0000-000000000000" \
    --docker-password="$ACR_TOKEN" \
    -n "$NAMESPACE"

# Patch the default SA so any pod that doesn't specify its own SA can pull.
kubectl patch serviceaccount default -n "$NAMESPACE" \
    -p "{\"imagePullSecrets\":[{\"name\":\"$ACR_PULL_SECRET_NAME\"}]}"

# Chart-created ServiceAccounts don't exist yet; helper below patches them
# right after each helm install so the pull secret takes effect before pods
# roll out.
attach_pull_secret_to_all_sas() {
    for sa in $(kubectl get serviceaccounts -n "$NAMESPACE" -o name); do
        kubectl patch "$sa" -n "$NAMESPACE" \
            -p "{\"imagePullSecrets\":[{\"name\":\"$ACR_PULL_SECRET_NAME\"}]}" \
            >/dev/null || true
    done
}

# Idempotency: drop any previous release before installing.
helm uninstall "$RELEASE_NAME" -n "$NAMESPACE" || true
# Prereq release (from the previous two-chart layout) may still exist on a
# recycled cluster; clean it up so it doesn't leave orphan resources.
helm uninstall "${PREREQ_RELEASE_NAME:-cache-server-prereq}" -n "$NAMESPACE" || true

# Single-chart install: `tachyon-cache` bundles the manager (control plane)
# and cache-server (data plane). `--set (global.)imagePullSecrets[0].name`
# covers charts that read it directly; the SA-patch + pod-delete dance below
# covers charts that don't.
echo "Deploying tachyon-cache helm chart from $CACHE_SERVER_CHART_REF ..."
helm install "$RELEASE_NAME" "$CACHE_SERVER_CHART_REF" \
    --version "$CACHE_SERVER_CHART_VERSION" \
    -n "$NAMESPACE" \
    --set "global.imagePullSecrets[0].name=$ACR_PULL_SECRET_NAME" \
    --set "imagePullSecrets[0].name=$ACR_PULL_SECRET_NAME" \
    --set manager.image.repository="${CACHE_CONTROLLER_IMAGE%:*}" \
    --set manager.image.tag="${CACHE_CONTROLLER_IMAGE#*:}" \
    --set manager.cacheServerImage.repository="${CACHE_SERVER_IMAGE%:*}" \
    --set manager.cacheServerImage.tag="${CACHE_SERVER_IMAGE#*:}"

echo "Attaching ACR pull secret to tachyon-cache ServiceAccounts ..."
attach_pull_secret_to_all_sas
kubectl delete pods -n "$NAMESPACE" --all --wait=false >/dev/null || true

# --- Wait for pods to come up ---------------------------------------------

# Bounded wait so an ImagePull / scheduling failure surfaces instead of hanging.
CACHE_SERVER_READY_TIMEOUT="${CACHE_SERVER_READY_TIMEOUT:-600}"
echo "Waiting up to ${CACHE_SERVER_READY_TIMEOUT}s for cacheserver pods to reach Running..."
deadline=$(( $(date +%s) + CACHE_SERVER_READY_TIMEOUT ))
while true; do
    ready_pods=$(kubectl get pods -n "$NAMESPACE" -l app=cacheserver --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l)
    total_pods=$(kubectl get pods -n "$NAMESPACE" -l app=cacheserver --no-headers 2>/dev/null | wc -l)
    if [[ "$ready_pods" -eq "$total_pods" && "$total_pods" -gt 0 ]]; then
        echo "All $total_pods cacheserver pod(s) are Running."
        break
    fi
    if [[ $(date +%s) -ge $deadline ]]; then
        echo "ERROR: cacheserver pods did not reach Running within ${CACHE_SERVER_READY_TIMEOUT}s" \
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
for pod in $(kubectl get pods -n "$NAMESPACE" -l app=cacheserver -o jsonpath='{.items[*].metadata.name}'); do
    echo "  $pod.cacheserver.$NAMESPACE.svc.cluster.local:$CACHE_SERVER_PORT"
done

echo ""
echo "Cache server deployed successfully!"
