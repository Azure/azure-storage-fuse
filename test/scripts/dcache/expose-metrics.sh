#!/bin/bash
#
# Expose each cacheserver pod's Prometheus metrics port
# ($CACHE_SERVER_METRICS_PORT, default 9096) on a sequential localhost port
# using `kubectl port-forward`, so the dist_cache-focused E2E Go tests
# (test/dcache_e2e/) can scrape hit/upload counters from the host.
#
# This is a small sibling of expose-cacheserver.sh (which forwards the
# gRPC/wire port). It intentionally lives in a separate file + writes to
# separate output files so that:
#   * The existing e2e-tests.yml path (which does not need metrics) is
#     unaffected.
#   * teardown-kind.sh can kill both PID lists uniformly.
#
# Outputs:
#   $DCACHE_METRICS_ENDPOINTS_FILE - single line, comma-separated URL list,
#                                    e.g. "http://localhost:9096,http://localhost:9097,..."
#   $DCACHE_METRICS_PORTFORWARD_PIDS_FILE - one PID per line for teardown.

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
DCACHE_METRICS_ENDPOINTS_FILE="${DCACHE_METRICS_ENDPOINTS_FILE:-$DEFAULT_OUTPUT_DIR/dcache_metrics_endpoints.txt}"
DCACHE_METRICS_PORTFORWARD_PIDS_FILE="${DCACHE_METRICS_PORTFORWARD_PIDS_FILE:-$DEFAULT_OUTPUT_DIR/dcache_metrics_portforward_pids.txt}"

# Local port to start allocating metrics forwards from. Kept distinct from
# CACHE_SERVER_PORT (default 9065) to avoid clashing with the wire port list.
METRICS_LOCAL_PORT_START="${METRICS_LOCAL_PORT_START:-$CACHE_SERVER_METRICS_PORT}"

mkdir -p "$(dirname "$DCACHE_METRICS_ENDPOINTS_FILE")" "$(dirname "$DCACHE_METRICS_PORTFORWARD_PIDS_FILE")"

: > "$DCACHE_METRICS_ENDPOINTS_FILE"
: > "$DCACHE_METRICS_PORTFORWARD_PIDS_FILE"

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

endpoints=""
local_port="$METRICS_LOCAL_PORT_START"

for pod in $pods; do
    echo "Starting kubectl port-forward for pod/$pod (metrics) -> localhost:$local_port"
    kubectl port-forward -n "$NAMESPACE" "pod/$pod" "$local_port:$CACHE_SERVER_METRICS_PORT" >/dev/null 2>&1 &
    pf_pid=$!
    echo "$pf_pid" >> "$DCACHE_METRICS_PORTFORWARD_PIDS_FILE"

    if ! wait_for_port "$local_port" 30; then
        echo "ERROR: metrics port-forward for pod/$pod (pid=$pf_pid) never came up" >&2
        exit 1
    fi

    if [[ -z "$endpoints" ]]; then
        endpoints="http://localhost:$local_port"
    else
        endpoints="$endpoints,http://localhost:$local_port"
    fi

    local_port=$((local_port + 1))
done

echo -n "$endpoints" > "$DCACHE_METRICS_ENDPOINTS_FILE"

echo ""
echo "Metrics endpoints: $endpoints"
echo "Wrote endpoints to:      $DCACHE_METRICS_ENDPOINTS_FILE"
echo "Wrote port-forward PIDs: $DCACHE_METRICS_PORTFORWARD_PIDS_FILE"
