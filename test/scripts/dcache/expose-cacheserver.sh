#!/bin/bash
#
# Expose each cacheserver pod on a sequential localhost port using
# `kubectl port-forward`, so blobfuse2 (running on the pipeline host, not inside
# the cluster) can populate dist_cache.server-list with reachable endpoints.
#
# Outputs:
#   $DCACHE_SERVER_LIST_FILE   - single line, comma-separated host:port list
#   $DCACHE_PORTFORWARD_PIDS_FILE - one PID per line, so teardown can kill them

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load shared configuration
CONFIG_FILE="$SCRIPT_DIR/config/nightly.config"
if [[ -f "$CONFIG_FILE" ]]; then
    # shellcheck source=./config/nightly.config
    source "$CONFIG_FILE"
    echo "Loaded config from: $CONFIG_FILE"
fi

DEFAULT_OUTPUT_DIR="${AGENT_TEMPDIRECTORY:-/tmp}"
DCACHE_SERVER_LIST_FILE="${DCACHE_SERVER_LIST_FILE:-$DEFAULT_OUTPUT_DIR/dcache_server_list.txt}"
DCACHE_PORTFORWARD_PIDS_FILE="${DCACHE_PORTFORWARD_PIDS_FILE:-$DEFAULT_OUTPUT_DIR/dcache_portforward_pids.txt}"

mkdir -p "$(dirname "$DCACHE_SERVER_LIST_FILE")" "$(dirname "$DCACHE_PORTFORWARD_PIDS_FILE")"

: > "$DCACHE_SERVER_LIST_FILE"
: > "$DCACHE_PORTFORWARD_PIDS_FILE"

# Wait until $port on localhost is accepting TCP connections, up to $timeout seconds.
wait_for_port() {
    local port="$1"
    local timeout="${2:-30}"
    local waited=0
    while ! nc -z localhost "$port" 2>/dev/null; do
        if [[ "$waited" -ge "$timeout" ]]; then
            echo "ERROR: port $port not listening after ${timeout}s" >&2
            return 1
        fi
        sleep 1
        waited=$((waited + 1))
    done
}

pods=$(kubectl get pods -n "$NAMESPACE" -l app=cacheserver -o jsonpath='{.items[*].metadata.name}')
if [[ -z "$pods" ]]; then
    echo "ERROR: no cacheserver pods found in namespace '$NAMESPACE'" >&2
    exit 1
fi

server_list=""
local_port="$CACHE_SERVER_PORT"

for pod in $pods; do
    echo "Starting kubectl port-forward for pod/$pod -> localhost:$local_port"
    kubectl port-forward -n "$NAMESPACE" "pod/$pod" "$local_port:$CACHE_SERVER_PORT" >/dev/null 2>&1 &
    pf_pid=$!
    echo "$pf_pid" >> "$DCACHE_PORTFORWARD_PIDS_FILE"

    if ! wait_for_port "$local_port" 30; then
        echo "ERROR: port-forward for pod/$pod (pid=$pf_pid) never came up" >&2
        exit 1
    fi

    if [[ -z "$server_list" ]]; then
        server_list="localhost:$local_port"
    else
        server_list="$server_list,localhost:$local_port"
    fi

    local_port=$((local_port + 1))
done

echo -n "$server_list" > "$DCACHE_SERVER_LIST_FILE"

echo ""
echo "Server list: $server_list"
echo "Wrote server list to:    $DCACHE_SERVER_LIST_FILE"
echo "Wrote port-forward PIDs: $DCACHE_PORTFORWARD_PIDS_FILE"
