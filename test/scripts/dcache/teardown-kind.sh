#!/bin/bash
#
# Best-effort teardown for the dist_cache nightly E2E stage. Intended to be
# called from an ADO step with condition: always(), so it MUST NOT abort on
# any single failure -- hence `set +e` instead of `set -euo pipefail`.

set +e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

CONFIG_FILE="$SCRIPT_DIR/config/nightly.config"
if [[ -f "$CONFIG_FILE" ]]; then
    # shellcheck source=./config/nightly.config
    source "$CONFIG_FILE"
    echo "Loaded config from: $CONFIG_FILE"
fi

CLUSTER_NAME="${CLUSTER_NAME:-blobfuse-dcache}"
DEFAULT_OUTPUT_DIR="${AGENT_TEMPDIRECTORY:-/tmp}"
DCACHE_PORTFORWARD_PIDS_FILE="${DCACHE_PORTFORWARD_PIDS_FILE:-$DEFAULT_OUTPUT_DIR/dcache_portforward_pids.txt}"

# --- Kill port-forwards ----------------------------------------------------
if [[ -f "$DCACHE_PORTFORWARD_PIDS_FILE" ]]; then
    echo "Killing port-forward PIDs from $DCACHE_PORTFORWARD_PIDS_FILE ..."
    while read -r pid; do
        [[ -z "$pid" ]] && continue
        echo "  kill $pid"
        kill "$pid" 2>/dev/null
    done < "$DCACHE_PORTFORWARD_PIDS_FILE"
    rm -f "$DCACHE_PORTFORWARD_PIDS_FILE"
fi

# --- Helm uninstall --------------------------------------------------------
echo "Uninstalling helm release '$RELEASE_NAME' from namespace '$NAMESPACE'..."
helm uninstall "$RELEASE_NAME" -n "$NAMESPACE" || true
echo "Uninstalling helm release '$PREREQ_RELEASE_NAME' from namespace '$NAMESPACE'..."
helm uninstall "$PREREQ_RELEASE_NAME" -n "$NAMESPACE" || true

# --- Delete namespace ------------------------------------------------------
echo "Deleting namespace '$NAMESPACE' (async)..."
kubectl delete namespace "$NAMESPACE" --wait=false || true

# --- Delete cluster --------------------------------------------------------
echo "Deleting kind cluster '$CLUSTER_NAME'..."
kind delete cluster --name "$CLUSTER_NAME" || true

echo "Teardown complete."
