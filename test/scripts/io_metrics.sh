#!/bin/bash

# Records application throughput and peak network rates for one test case.
# Usage: measure_io_case <metrics.tsv> <case> <operation> <bytes> <command> [args...]

_io_metrics_interface() {
    if [ -n "${IO_METRICS_INTERFACE:-}" ]; then
        printf '%s\n' "$IO_METRICS_INTERFACE"
        return
    fi

    ip -o route show default 2>/dev/null | awk '{print $5; exit}'
}

_io_metrics_stat() {
    local interface="$1"
    local stat="$2"
    local path="/sys/class/net/$interface/statistics/$stat"

    if [ -n "$interface" ] && [ -r "$path" ]; then
        cat "$path"
    else
        printf '0\n'
    fi
}

_io_metrics_sample_network() {
    local interface="$1"
    local stop_file="$2"
    local result_file="$3"
    local previous_rx previous_tx previous_ns
    local max_rx=0
    local max_tx=0

    previous_rx=$(_io_metrics_stat "$interface" rx_bytes)
    previous_tx=$(_io_metrics_stat "$interface" tx_bytes)
    previous_ns=$(date +%s%N)

    while [ ! -e "$stop_file" ]; do
        sleep 0.25
        local current_rx current_tx current_ns elapsed_ns rx_rate tx_rate
        current_rx=$(_io_metrics_stat "$interface" rx_bytes)
        current_tx=$(_io_metrics_stat "$interface" tx_bytes)
        current_ns=$(date +%s%N)
        elapsed_ns=$((current_ns - previous_ns))

        if [ "$elapsed_ns" -gt 0 ]; then
            rx_rate=$(((current_rx - previous_rx) * 1000000000 / elapsed_ns))
            tx_rate=$(((current_tx - previous_tx) * 1000000000 / elapsed_ns))
            [ "$rx_rate" -gt "$max_rx" ] && max_rx=$rx_rate
            [ "$tx_rate" -gt "$max_tx" ] && max_tx=$tx_rate
        fi

        previous_rx=$current_rx
        previous_tx=$current_tx
        previous_ns=$current_ns
    done

    printf '%s\t%s\n' "$max_rx" "$max_tx" > "$result_file"
}

measure_io_case() {
    local metrics_file="$1"
    local case_name="$2"
    local operation="$3"
    local bytes="$4"
    shift 4

    local interface temp_dir stop_file network_file sampler_pid
    local start_ns end_ns elapsed_ns result max_rx max_tx
    interface=$(_io_metrics_interface)
    temp_dir=$(mktemp -d)
    stop_file="$temp_dir/stop"
    network_file="$temp_dir/network.tsv"

    _io_metrics_sample_network "$interface" "$stop_file" "$network_file" &
    sampler_pid=$!
    start_ns=$(date +%s%N)

    result=0
    "$@" || result=$?

    end_ns=$(date +%s%N)
    touch "$stop_file"
    wait "$sampler_pid"
    read -r max_rx max_tx < "$network_file"
    elapsed_ns=$((end_ns - start_ns))
    [ "$elapsed_ns" -le 0 ] && elapsed_ns=1

    mkdir -p "$(dirname "$metrics_file")"
    case_name=${case_name//$'\t'/ }
    case_name=${case_name//$'\n'/ }
    (
        flock 9
        printf '%s\t%s\t%s\t%s\t%s\t%s\n' \
            "$case_name" "$operation" "$bytes" "$elapsed_ns" "${max_rx:-0}" "${max_tx:-0}" >&9
    ) 9>>"$metrics_file"

    rm -rf "$temp_dir"
    return "$result"
}