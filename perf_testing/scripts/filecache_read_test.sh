#!/bin/bash
set -e

# Custom read test for filecache mode
# This test measures the true throughput including file open time (when the file is downloaded in filecache)
# Usage: ./filecache_read_test.sh <mount_dir> <cache_mode>

MOUNT_DIR="$1"
CACHE_MODE="$2"
OUTPUT_DIR="./read"
TEST_FILE_SIZE="100G"
BLOCK_SIZE="1M"
TEST_FILE="filecache_read_test_100gb.data"

# Validate input
if [[ -z "$MOUNT_DIR" || -z "$CACHE_MODE" ]]; then
    echo "Usage: $0 <mount_dir> <cache_mode>"
    exit 1
fi

# Only run this test for file_cache mode
if [[ "$CACHE_MODE" != "file_cache" ]]; then
    echo "Skipping filecache read test - only runs for file_cache mode"
    exit 0
fi

# Ensure output directory exists
mkdir -p "${OUTPUT_DIR}"
chmod 777 "${OUTPUT_DIR}"

# Blobfuse settings
LOG_TYPE="syslog"
LOG_LEVEL="log_err"

# --------------------------------------------------------------------------------------------------
# Helper: Unmount and cleanup
cleanup_mount() {
    set +e
    blobfuse2 unmount all > /dev/null 2>&1
    sleep 5
    set -e
}

# Helper: Mount blobfuse and wait for system to stabilize
mount_blobfuse() {
    echo "Mounting blobfuse on ${MOUNT_DIR}..."
    
    cleanup_mount

    # Clear mount directory and temp cache before mounting
    rm -rf "${MOUNT_DIR:?}/"* 2>/dev/null || true
    if [ -d "/mnt/localssd/tempcache" ]; then
        rm -rf /mnt/localssd/tempcache/* 2>/dev/null || true
    fi

    set +e
    blobfuse2 mount "${MOUNT_DIR}" \
        --config-file=./config.yaml \
        --log-type="${LOG_TYPE}" \
        --log-level="${LOG_LEVEL}"
    
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

# --------------------------------------------------------------------------------------------------
# Main Execution

echo "Starting filecache read test with 100GB file..."

# Step 1: Mount blobfuse and create the test file if it doesn't exist
mount_blobfuse

if [ ! -f "${MOUNT_DIR}/${TEST_FILE}" ]; then
    echo "Creating 100GB test file: ${TEST_FILE}..."
    echo "This may take some time..."
    
    # Create the file using dd (this will upload to blob storage in file_cache mode)
    dd if=/dev/urandom of="${MOUNT_DIR}/${TEST_FILE}" bs=1M count=102400 status=progress
    
    echo "Test file created successfully."
    
    # Sync to ensure file is uploaded
    sync
    sleep 5
fi

# Verify file exists and get its size
if [ ! -f "${MOUNT_DIR}/${TEST_FILE}" ]; then
    echo "Error: Test file does not exist"
    exit 1
fi

FILE_SIZE=$(stat -c%s "${MOUNT_DIR}/${TEST_FILE}")
echo "Test file size: $FILE_SIZE bytes"

# Step 2: Unmount to clear any cached data
echo "Unmounting to clear cache..."
cleanup_mount
sleep 5

# Step 3: Clear kernel page cache
echo "Dropping kernel caches..."
sudo sh -c "echo 3 > /proc/sys/vm/drop_caches"
sleep 2

# Step 4: Mount again for the read test
echo "Remounting for read test..."
mount_blobfuse

# Step 5: Run the read test with direct I/O
echo "Starting sequential read test with direct I/O (block size: ${BLOCK_SIZE})..."

# Set the network interface to monitor
INTERFACE="eth0"

# Get initial network stats
start_rx=$(cat /sys/class/net/$INTERFACE/statistics/rx_bytes)
start_tx=$(cat /sys/class/net/$INTERFACE/statistics/tx_bytes)
start_time=$(date +%s.%N)

# Perform the read test using dd with direct I/O
# O_DIRECT flag ensures we bypass the OS page cache
# This includes the open time (when filecache downloads the file)
dd if="${MOUNT_DIR}/${TEST_FILE}" of=/dev/null bs=${BLOCK_SIZE} iflag=direct 2>&1 | tee "${OUTPUT_DIR}/dd_output.txt"

# Get final stats
end_time=$(date +%s.%N)
end_rx=$(cat /sys/class/net/$INTERFACE/statistics/rx_bytes)
end_tx=$(cat /sys/class/net/$INTERFACE/statistics/tx_bytes)

# Calculate metrics
duration=$(echo "$end_time - $start_time" | bc)
rx_bytes=$((end_rx - start_rx))
tx_bytes=$((end_tx - start_tx))

# Calculate bandwidth in Mbps
rx_mbps=$(echo "scale=4; ($rx_bytes * 8) / ($duration * 1000000)" | bc)
tx_mbps=$(echo "scale=4; ($tx_bytes * 8) / ($duration * 1000000)" | bc)

# Calculate throughput in MiB/s (includes open time)
throughput_mibs=$(echo "scale=4; $FILE_SIZE / ($duration * 1024 * 1024)" | bc)

echo "-------------------------------------"
echo "Test completed!"
echo "Duration: ${duration} seconds"
echo "Throughput: ${throughput_mibs} MiB/s (including file open time)"
echo
echo "Network Statistics:"
echo "Interface: $INTERFACE"
echo "Received (RX): $rx_bytes bytes (${rx_mbps} Mbps)"
echo "Transmitted (TX): $tx_bytes bytes (${tx_mbps} Mbps)"
echo "-------------------------------------"

# Generate JSON output for bandwidth results
cat > "${OUTPUT_DIR}/filecache_sequential_read_bandwidth_summary.json" <<EOF
{
  "name": "filecache_sequential_read_100GB_directio",
  "value": ${throughput_mibs},
  "unit": "MiB/s"
}
EOF

# Generate JSON output for latency (time taken)
latency_ms=$(echo "scale=4; $duration * 1000" | bc)
cat > "${OUTPUT_DIR}/filecache_sequential_read_latency_summary.json" <<EOF
{
  "name": "filecache_sequential_read_100GB_directio",
  "value": ${latency_ms},
  "unit": "milliseconds"
}
EOF

# Update the final results files
if [ -f "${OUTPUT_DIR}/bandwidth_results.json" ]; then
    # Append to existing results
    jq -s '.[0] + .[1]' "${OUTPUT_DIR}/bandwidth_results.json" "${OUTPUT_DIR}/filecache_sequential_read_bandwidth_summary.json" > "${OUTPUT_DIR}/bandwidth_results_tmp.json"
    mv "${OUTPUT_DIR}/bandwidth_results_tmp.json" "${OUTPUT_DIR}/bandwidth_results.json"
else
    # Create new results file
    jq -n '[inputs]' "${OUTPUT_DIR}/filecache_sequential_read_bandwidth_summary.json" > "${OUTPUT_DIR}/bandwidth_results.json"
fi

if [ -f "${OUTPUT_DIR}/latency_results.json" ]; then
    # Append to existing results
    jq -s '.[0] + .[1]' "${OUTPUT_DIR}/latency_results.json" "${OUTPUT_DIR}/filecache_sequential_read_latency_summary.json" > "${OUTPUT_DIR}/latency_results_tmp.json"
    mv "${OUTPUT_DIR}/latency_results_tmp.json" "${OUTPUT_DIR}/latency_results.json"
else
    # Create new results file
    jq -n '[inputs]' "${OUTPUT_DIR}/filecache_sequential_read_latency_summary.json" > "${OUTPUT_DIR}/latency_results.json"
fi

echo "Results saved to ${OUTPUT_DIR}/bandwidth_results.json and ${OUTPUT_DIR}/latency_results.json"

# Cleanup
cleanup_mount

echo "Filecache read test completed successfully!"
