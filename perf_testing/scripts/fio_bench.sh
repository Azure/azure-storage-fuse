#!/bin/bash
set -e

# Configuration
ITERATIONS=3
MOUNT_DIR="$1"
TEST_NAME="$2"  # Expect "read" or "write"
CACHE_MODE="$3" # Expect "block_cache" or "file_cache"
OUTPUT_DIR="./${TEST_NAME}"

# Blobfuse settings
LOG_TYPE="syslog"
LOG_LEVEL="log_err"
CACHE_PATH="" # Set if needed, e.g., "--block-cache-path=/mnt/tempcache"

# Validate input
if [[ -z "$MOUNT_DIR" || -z "$TEST_NAME" ]]; then
    echo "Usage: $0 <mount_dir> <test_name> <cache_mode>"
    echo "  test_name must be 'read' or 'write'"
    echo "  cache_mode must be 'block_cache' or 'file_cache'"
    exit 1
fi

if [[ "$TEST_NAME" != "read" && "$TEST_NAME" != "write" ]]; then
    echo "Invalid test name. Please provide either 'read' or 'write'."
    exit 1
fi

# Ensure output directory exists
mkdir -p "${OUTPUT_DIR}"
chmod 777 "${OUTPUT_DIR}"

# --------------------------------------------------------------------------------------------------
# Helper: Unmount and cleanup
cleanup_mount() {
    set +e
    blobfuse2 unmount all > /dev/null 2>&1
    sleep 5
    # Optional: cleanup local cache if needed
    # rm -rf ~/.blobfuse2/*
    set -e
}

# Helper: Mount blobfuse and wait for system to stabilize
mount_blobfuse() {
    echo "Mounting blobfuse on ${MOUNT_DIR}..."
    
    cleanup_mount

    # Clear mount directory and temp cache before mounting
    rm -rf "${MOUNT_DIR:?}/"* 2>/dev/null || true
    if [ -d "/mnt/tempcache" ]; then
        rm -rf /mnt/tempcache/* 2>/dev/null || true
    fi

    set +e
    blobfuse2 mount "${MOUNT_DIR}" \
        --config-file=./config.yaml \
        --log-type="${LOG_TYPE}" \
        --log-level="${LOG_LEVEL}" \
        ${CACHE_PATH}
    
    local status=$?
    set -e

    if [ $status -ne 0 ]; then
        echo "Error: Failed to mount file system."
        exit 1
    fi

    ps aux | grep '[b]lobfuse2'

    # Wait for daemon to stabilize
    sleep 10

    if ! df -h | grep -q blobfuse; then
        echo "Error: blobfuse mount not found in df output."
        exit 1
    fi
    
    echo "File system mounted successfully."
}

# Set the network interface to monitor
INTERFACE="eth0"

# Helper: Execute a single FIO job multiple times
run_fio_job() {
    local job_file=$1
    local job_name
    job_name=$(basename "${job_file}" .fio)

    echo -n "Running job ${job_name} ... "

    # drop the kernel page cache to get more accurate results
    sudo sh -c "echo 3 > /proc/sys/vm/drop_caches"

    # Get Network initial stats for logs.
    start_rx=$(cat /sys/class/net/$INTERFACE/statistics/rx_bytes)
    start_tx=$(cat /sys/class/net/$INTERFACE/statistics/tx_bytes)
    start_time=$(date +%s.%N)

    set +e
    timeout 300m fio --thread \
            --output="${OUTPUT_DIR}/${job_name}_trial.json" \
            --output-format=json \
            --directory="${MOUNT_DIR}" \
            --eta=never \
            "${job_file}" > /dev/null

    local status=$?
    set -e

    if [ $status -ne 0 ]; then
        echo "Error: Job ${job_name} failed with status ${status}"
        exit 1
    fi

    # Get final stats for network usage calculation
    end_time=$(date +%s.%N)
    end_rx=$(cat /sys/class/net/$INTERFACE/statistics/rx_bytes)
    end_tx=$(cat /sys/class/net/$INTERFACE/statistics/tx_bytes)

    echo "-------------------------------------"
    echo "Command finished. Calculating network usage..."

    # Calculate the duration
    duration=$(echo "$end_time - $start_time" | bc)

    # Calculate the difference in bytes
    rx_bytes=$((end_rx - start_rx))
    tx_bytes=$((end_tx - start_tx))

    # Calculate bandwidth in Megabits per second (Mbps)
    # (bytes * 8) / (duration * 1000 * 1000)
    rx_mbps=$(echo "scale=4; ($rx_bytes * 8) / ($duration * 1000000)" | bc)
    tx_mbps=$(echo "scale=4; ($tx_bytes * 8) / ($duration * 1000000)" | bc)

    # Output the results
    echo
    echo "Interface: $INTERFACE"
    echo "Duration: ${duration} seconds"
    echo
    echo "Received (RX):"
    echo "  - Bytes: $rx_bytes"
    echo "  - Average Bandwidth: ${rx_mbps} Mbps"
    echo
    echo "Transmitted (TX):"
    echo "  - Bytes: $tx_bytes"
    echo "  - Average Bandwidth: ${tx_mbps} Mbps"

    # done
    echo "Done."

    cat "${OUTPUT_DIR}/${job_name}_trial.json"

    # Generate summary JSONs using jq
    # Bandwidth Summary
    jq -n 'reduce inputs.jobs[] as $job (null; .name = $job.jobname | .len += 1 | .value += (
        if ($job."job options".rw | contains("read")) then $job.read.bw / 1024
        else $job.write.bw / 1024 end
    )) | {name: .name, value: (.value / .len), unit: "MiB/s"}' "${OUTPUT_DIR}/${job_name}_trial".json | tee "${OUTPUT_DIR}/${job_name}_bandwidth_summary.json" > /dev/null

    # Latency Summary
    jq -n 'reduce inputs.jobs[] as $job (null; .name = $job.jobname | .len += 1 | .value += (
        if ($job."job options".rw | contains("read")) then $job.read.lat_ns.mean / 1000000
        else $job.write.lat_ns.mean / 1000000 end
    )) | {name: .name, value: (.value / .len), unit: "milliseconds"}' "${OUTPUT_DIR}/${job_name}_trial".json | tee "${OUTPUT_DIR}/${job_name}_latency_summary.json" > /dev/null
}

# Helper: Iterate over all FIO files in a directory
run_test_suite() {
    local config_dir=$1
    echo "Starting test suite from: ${config_dir}"

    for job_file in "${config_dir}"/*.fio; do
        if [ ! -f "$job_file" ]; then continue; fi
        
        mount_blobfuse
	rm -rf "${MOUNT_DIR}/"*
        run_fio_job "$job_file"
        cleanup_mount
    done
}

# --------------------------------------------------------------------------------------------------
# Main Execution

cleanup_mount

if [[ "${TEST_NAME}" == "write" ]]; then
    run_test_suite "./perf_testing/config/write"
elif [[ "${TEST_NAME}" == "read" ]]; then
    run_test_suite "./perf_testing/config/read"
fi

# Final Reporting
echo "Generating final reports..."
jq -n '[inputs]' "${OUTPUT_DIR}"/*_bandwidth_summary.json | tee "${OUTPUT_DIR}/bandwidth_results.json"
jq -n '[inputs]' "${OUTPUT_DIR}"/*_latency_summary.json | tee "${OUTPUT_DIR}/latency_results.json"

echo "Test complete. Results saved in ${OUTPUT_DIR}"
