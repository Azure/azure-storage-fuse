#!/bin/bash

# ==========================================
# CONFIGURATION
# ==========================================
MOUNT_POINT="${FUSE_MOUNT_POINT:-/tmp/fuse_mount}"
TEST_DIR="$MOUNT_POINT/test_stress_$(date +%s)"
REF_DIR="/tmp/fuse_test_refs_$(date +%s)"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# ==========================================
# SETUP & TEARDOWN
# ==========================================

setup() {
    if [ ! -d "$MOUNT_POINT" ]; then
        echo -e "${RED}Error: Mount point $MOUNT_POINT does not exist.${NC}"
        exit 1
    fi
    mkdir -p "$TEST_DIR"
    mkdir -p "$REF_DIR"
    echo -e "${YELLOW}Starting tests on: $TEST_DIR${NC}"
    echo "------------------------------------------------"
}

cleanup() {
    echo "------------------------------------------------"
    echo -e "${YELLOW}Cleaning up...${NC}"
    rm -rf "$TEST_DIR"
    rm -rf "$REF_DIR"
    echo "Done."
}

trap cleanup EXIT

# ==========================================
# HELPER FUNCTIONS
# ==========================================

# Usage: run_integrity_test <test_name> <block_size> <count> <extra_dd_args>
run_integrity_test() {
    local name="$1"
    local bs="$2"
    local count="$3"
    local extra_args="$4"
    
    local filename="${name}.dat"
    local ref_file="$REF_DIR/$filename"
    local fuse_file="$TEST_DIR/$filename"

    echo -n "TEST: $name (bs=$bs count=$count args='$extra_args') ... "

    # 1. Generate random source data on local disk
    dd if=/dev/urandom of="$ref_file" bs="$bs" count="$count" status=none

    # 2. Write to FUSE
    dd if="$ref_file" of="$fuse_file" bs="$bs" count="$count" $extra_args status=none conv=fsync
    if [ $? -ne 0 ]; then
        echo -e "${RED}WRITE FAILED${NC}"
        return 1
    fi

    # 3. Read back from FUSE to check integrity
    # We compare the MD5 of the reference file vs the file on FUSE
    local ref_md5=$(md5sum "$ref_file" | awk '{print $1}')
    local fuse_md5=$(md5sum "$fuse_file" | awk '{print $1}')

    if [ "$ref_md5" == "$fuse_md5" ]; then
        echo -e "${GREEN}PASS${NC}"
    else
        echo -e "${RED}FAIL (Checksum Mismatch)${NC}"
        echo "  Ref : $ref_md5"
        echo "  FUSE: $fuse_md5"
        exit 1
    fi
}

# ==========================================
# TEST SCENARIOS
# ==========================================

setup

# --- SCENARIO 1: Requested Block Sizes ---
# Standard sequential writes
echo -e "${YELLOW}[Scenario 1] Standard Block Sizes${NC}"
run_integrity_test "std_128k" "128k" "100"  # 12.8 MB
run_integrity_test "std_128k" "256K" "1000" # 128 MB
run_integrity_test "std_1M"   "1M"   "20"  # 20 MB
run_integrity_test "std_5M"   "5M"  "10"   # 50 MB
run_integrity_test "std_10M"  "10M"  "5"   # 50 MB
run_integrity_test "std_10M" "10M" "100"   # 1000 MB

# --- SCENARIO 2: Odd/Unaligned Block Sizes ---
# FUSE buffers are often 4k aligned. Writing odd bytes tests buffer boundaries.
echo -e "\n${YELLOW}[Scenario 2] Unaligned/Odd Block Sizes${NC}"
run_integrity_test "odd_1023bytes" "1023" "1000" # Non-power of 2
run_integrity_test "odd_13bytes"   "13"   "5000" # Very small writes

# --- SCENARIO 3: Direct I/O ---
# 'oflag=direct' bypasses the Linux Page Cache. 
# This forces the FUSE file system to handle the exact write size immediately.
echo -e "\n${YELLOW}[Scenario 3] Direct I/O (Bypassing Kernel Cache)${NC}"
run_integrity_test "direct_4k" "4k" "500" "oflag=direct"
run_integrity_test "direct_1M" "1M" "10"  "oflag=direct"

# --- SCENARIO 4: Seek and Overwrite (Read-Modify-Write) ---
# This writes a file, then seeks into the middle and overwrites part of it.
echo -e "\n${YELLOW}[Scenario 4] Seek and Overwrite${NC}"

TEST_NAME="seek_overwrite"
REF_FILE="$REF_DIR/${TEST_NAME}.dat"
FUSE_FILE="$TEST_DIR/${TEST_NAME}.dat"

echo -n "TEST: Overwriting middle of file ... "

# Create base file (10MB)
dd if=/dev/urandom of="$REF_FILE" bs=1M count=10 status=none
cp "$REF_FILE" "$FUSE_FILE"

# Create a patch (1MB)
dd if=/dev/urandom of="$REF_DIR/patch.dat" bs=1M count=1 status=none

# Apply patch to middle of REF file (using seek=5)
dd if="$REF_DIR/patch.dat" of="$REF_FILE" bs=1M seek=5 count=1 conv=notrunc status=none

# Apply patch to middle of FUSE file
dd if="$REF_DIR/patch.dat" of="$FUSE_FILE" bs=1M seek=5 count=1 conv=notrunc status=none

# Verify
ref_md5=$(md5sum "$REF_FILE" | awk '{print $1}')
fuse_md5=$(md5sum "$FUSE_FILE" | awk '{print $1}')

if [ "$ref_md5" == "$fuse_md5" ]; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${RED}FAIL${NC}"
    exit 1
fi

# --- SCENARIO 5: Appending ---
# Tests if the file size updates correctly when appending.
echo -e "\n${YELLOW}[Scenario 5] Appending Data${NC}"

TEST_NAME="append_test"
REF_FILE="$REF_DIR/${TEST_NAME}.dat"
FUSE_FILE="$TEST_DIR/${TEST_NAME}.dat"

echo -n "TEST: Appending to file ... "

# Initial write
dd if=/dev/urandom of="$REF_FILE" bs=1M count=1 status=none
cp "$REF_FILE" "$FUSE_FILE"

# Generate data to append
dd if=/dev/urandom of="$REF_DIR/append.dat" bs=1M count=1 status=none

# Append to REF
cat "$REF_DIR/append.dat" >> "$REF_FILE"

# Append to FUSE using dd conv=notrunc oflag=append
dd if="$REF_DIR/append.dat" of="$FUSE_FILE" bs=1M count=1 oflag=append conv=notrunc status=none

# Verify
ref_md5=$(md5sum "$REF_FILE" | awk '{print $1}')
fuse_md5=$(md5sum "$FUSE_FILE" | awk '{print $1}')

if [ "$ref_md5" == "$fuse_md5" ]; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${RED}FAIL${NC}"
    exit 1
fi

# --- SCENARIO 6: Concurrency Stress Test ---
# Runs multiple writes in parallel to check for thread safety/locking issues.
echo -e "\n${YELLOW}[Scenario 6] Concurrency Stress Test${NC}"

pids=""
for i in {1..50}; do
    run_integrity_test "conc_job_$i" "1M" "100" &
    pids="$pids $!"
done

echo "Waiting for background jobs to finish..."
wait $pids
echo -e "${GREEN}Concurrency test batch completed.${NC}"

echo -e "\n${GREEN}ALL TESTS COMPLETED SUCCESSFULLY${NC}"
