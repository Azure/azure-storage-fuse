#!/bin/bash

# Usage: ./fio_fuse_integrity.sh /path/to/mountpoint
MOUNT_POINT="$1"

if [ -z "$MOUNT_POINT" ]; then
    echo "Usage: $0 /path/to/mountpoint"
    exit 1
fi

if [ ! -d "$MOUNT_POINT" ]; then
    echo "Error: Directory $MOUNT_POINT does not exist."
    exit 1
fi

TEST_FILE="$MOUNT_POINT/fio_integrity.dat"
SIZE="1000M" # Size of the file to test with
BLOCK_SIZES=("128k" "1M" "10M")

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

run_fio_test() {
    local bs="$1"
    local pattern="$2"
    local desc="$3"

    echo -e "${CYAN}------------------------------------------------------------${NC}"
    echo -e "${CYAN}Running: $desc${NC}"
    echo -e "${CYAN}Block Size: $bs | Pattern: $pattern${NC}"
    echo -e "${CYAN}------------------------------------------------------------${NC}"

    # FIO COMMAND EXPLANATION:
    # --rw=write        : Sequential write phase
    # --do_verify=1     : Perform a read-back verification phase after writing
    # --verify=md5      : Calculate MD5 of data written and compare on read
    # --verify_fatal=1  : Stop immediately if corruption is found
    # --fsync_on_close=1: Ensure data is flushed to FUSE backend before verify
    # --serialize_overlap=1: Prevents multiple jobs from writing to the same block at once (if we added threads)
    
    fio --name=integrity_test \
        --filename="$TEST_FILE" \
        --size="$SIZE" \
        --bs="$bs" \
        --rw="$pattern" \
        --verify=md5 \
        --do_verify=1 \
        --verify_fatal=1 \
        --verify_dump=1 \
        --fsync_on_close=1 \
        --invalidate=1 \
        --output-format=terse \
        --terse-version=3 > /dev/null

    if [ $? -eq 0 ]; then
        echo -e "${GREEN}[PASS] Integrity verified for $bs ($pattern)${NC}"
    else
        echo -e "${RED}[FAIL] CORRUPTION DETECTED in $bs ($pattern)${NC}"
        echo "Check the FUSE logs. FIO detected that read data did not match written data."
        exit 1
    fi
    
    # Clean up
    rm -f "$TEST_FILE"
}

echo "Starting FIO Integrity Suite on $MOUNT_POINT"

# ==============================================================================
# SCENARIO 1: Sequential Writes + Verify
# ==============================================================================
# This tests basic data pipe integrity. It writes the whole file, then reads it all back.
echo ""
echo "--- SCENARIO 1: Sequential Write & Verify ---"
for BS in "${BLOCK_SIZES[@]}"; do
    run_fio_test "$BS" "write" "Sequential Write Integrity"
done

# ==============================================================================
# SCENARIO 2: Random Writes + Verify
# ==============================================================================
# This is CRITICAL for FUSE. It writes blocks in random order.
# If your filesystem doesn't handle offsets correctly (e.g., lseek issues), 
# the verification will fail.
echo ""
echo "--- SCENARIO 2: Random Write & Verify ---"
for BS in "${BLOCK_SIZES[@]}"; do
    run_fio_test "$BS" "randwrite" "Random Write Integrity"
done

# ==============================================================================
# SCENARIO 3: Read-Write Mix
# ==============================================================================
# This simulates a database workload, doing reads and writes mixed together.
# It verifies the data "inline" as it goes.
echo ""
echo "--- SCENARIO 3: Mixed Read/Write (50/50) ---"
# We stick to 128k for this as it's more aggressive on IOPS
fio --name=mixed_rw_test \
    --filename="$TEST_FILE" \
    --size="$SIZE" \
    --bs="128k" \
    --rw=randrw \
    --rwmixread=50 \
    --verify=md5 \
    --verify_fatal=1 \
    --fsync=1 \
    --output-format=terse > /dev/null

if [ $? -eq 0 ]; then
    echo -e "${GREEN}[PASS] Mixed R/W Integrity verified${NC}"
else
    echo -e "${RED}[FAIL] Mixed R/W CORRUPTION DETECTED${NC}"
    exit 1
fi

# Clean up
rm -f "$TEST_FILE"

echo ""
echo -e "${GREEN}All FIO integrity tests passed!${NC}"
