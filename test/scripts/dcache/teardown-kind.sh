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

# --- Delete cluster --------------------------------------------------------
# `kind delete cluster` destroys the node containers wholesale, so per-release
# `helm uninstall` and namespace deletion are redundant. Skipping them saves
# ~30-60s on the always()-conditioned teardown path.
echo "Deleting kind cluster '$CLUSTER_NAME'..."
kind delete cluster --name "$CLUSTER_NAME" || true

echo "Teardown complete."
