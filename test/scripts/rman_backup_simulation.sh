#!/bin/bash

# rman_backup_simulation.sh
# Simulates Oracle RMAN backup workloads on a FUSE-mounted filesystem.
# Creates database files of various sizes, performs full and incremental
# backup simulations, and validates data integrity via MD5 checksums.
#
# Usage: ./rman_backup_simulation.sh /path/to/mountpoint /path/to/data_dir [sizes]
#   data_dir: directory where source data files will be created
#   sizes: comma-separated list of file sizes (default: 10M,100M,1G,10G)

MOUNT_POINT="$1"
DATA_DIR="$2"
SIZES="${3:-10M,100M,1G,10G}"

if [ -z "$MOUNT_POINT" ] || [ -z "$DATA_DIR" ]; then
    echo "Usage: $0 /path/to/mountpoint /path/to/data_dir [sizes]"
    echo "  data_dir: directory where source data files will be created"
    echo "  sizes: comma-separated list (e.g., 10M,100M,1G,10G)"
    exit 1
fi

if [ ! -d "$MOUNT_POINT" ]; then
    echo "Error: Directory $MOUNT_POINT does not exist."
    exit 1
fi

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

BACKUP_DIR="$MOUNT_POINT/rman_backup"
SOURCE_DIR="$DATA_DIR/rman_source_$$"
PASSED=0
FAILED=0

cleanup() {
    echo "Cleaning up..."
    rm -rf "$SOURCE_DIR"
    rm -rf "$BACKUP_DIR"
}

trap cleanup EXIT

mkdir -p "$SOURCE_DIR"
mkdir -p "$BACKUP_DIR"

IFS=',' read -ra SIZE_ARRAY <<< "$SIZES"

# ==============================================================================
# generate_datafile: Create a source file simulating an Oracle datafile
# ==============================================================================
generate_datafile() {
    local size="$1"
    local file="$SOURCE_DIR/datafile_${size}.dbf"

    echo -e "${CYAN}Generating source datafile: ${size}${NC}" >&2

    # Use dd with appropriate block size for the file size
    case "$size" in
        10M)  dd if=/dev/urandom of="$file" bs=1M count=10 status=none ;;
        100M) dd if=/dev/urandom of="$file" bs=1M count=100 status=none ;;
        1G)   dd if=/dev/urandom of="$file" bs=1M count=1024 status=none ;;
        10G)  dd if=/dev/urandom of="$file" bs=1M count=10240 status=none ;;
        *)    dd if=/dev/urandom of="$file" bs=1M count=10 status=none ;;
    esac

    if [ $? -ne 0 ]; then
        echo -e "${RED}[FAIL] Failed to generate datafile ${size}${NC}" >&2
        return 1
    fi

    echo "$file"
    return 0
}

# ==============================================================================
# full_backup: Simulate RMAN full backup (sequential write with large blocks)
# RMAN typically writes backup pieces using 256K or 1M block sizes.
# ==============================================================================
full_backup() {
    local src_file="$1"
    local size="$2"
    local bs="$3"
    local backup_file="$BACKUP_DIR/full_${size}_bs${bs}.bkp"

    echo -e "${CYAN}------------------------------------------------------------${NC}"
    echo -e "${CYAN}Full Backup: size=${size} block_size=${bs}${NC}"
    echo -e "${CYAN}------------------------------------------------------------${NC}"

    # Simulate RMAN full backup: sequential write with specified block size
    dd if="$src_file" of="$backup_file" bs="$bs" conv=fsync status=none
    if [ $? -ne 0 ]; then
        echo -e "${RED}[FAIL] Full backup write failed for ${size} bs=${bs}${NC}"
        FAILED=$((FAILED + 1))
        return 1
    fi

    # Drop page cache to force read from FUSE
    sudo sh -c 'echo 3 > /proc/sys/vm/drop_caches' 2>/dev/null

    # Verify integrity
    local src_md5
    local bkp_md5
    src_md5=$(md5sum "$src_file" | awk '{print $1}')
    bkp_md5=$(md5sum "$backup_file" | awk '{print $1}')

    if [ "$src_md5" == "$bkp_md5" ]; then
        echo -e "${GREEN}[PASS] Full backup integrity verified: ${size} bs=${bs}${NC}"
        echo "       MD5: $src_md5"
        PASSED=$((PASSED + 1))
    else
        echo -e "${RED}[FAIL] Full backup integrity MISMATCH: ${size} bs=${bs}${NC}"
        echo "       Source MD5:  $src_md5"
        echo "       Backup MD5:  $bkp_md5"
        FAILED=$((FAILED + 1))
        return 1
    fi

    rm -f "$backup_file"
    return 0
}

# ==============================================================================
# incremental_backup: Simulate RMAN incremental backup
# Writes partial blocks at various offsets to simulate changed-block tracking.
# ==============================================================================
incremental_backup() {
    local src_file="$1"
    local size="$2"
    local backup_file="$BACKUP_DIR/incr_${size}.bkp"

    echo -e "${CYAN}------------------------------------------------------------${NC}"
    echo -e "${CYAN}Incremental Backup Simulation: size=${size}${NC}"
    echo -e "${CYAN}------------------------------------------------------------${NC}"

    # First, create a full copy as the base
    cp "$src_file" "$backup_file"
    sync

    # Simulate incremental changes: overwrite specific blocks in the source
    local file_size_bytes
    file_size_bytes=$(stat -c%s "$src_file")

    # Write changed blocks at 3 different offsets (beginning, middle, near end)
    local offsets=("0" "$((file_size_bytes / 3))" "$((file_size_bytes * 2 / 3))")
    local block_size=1048576  # 1M blocks

    for offset in "${offsets[@]}"; do
        dd if=/dev/urandom of="$src_file" bs="$block_size" count=1 seek=$((offset / block_size)) conv=notrunc status=none
    done

    # Apply the incremental changes to the backup (simulate merge)
    for offset in "${offsets[@]}"; do
        dd if="$src_file" of="$backup_file" bs="$block_size" count=1 \
            skip=$((offset / block_size)) seek=$((offset / block_size)) conv=notrunc,fsync status=none
    done

    # Drop page cache
    sudo sh -c 'echo 3 > /proc/sys/vm/drop_caches' 2>/dev/null

    # Verify the merged backup matches the modified source
    local src_md5
    local bkp_md5
    src_md5=$(md5sum "$src_file" | awk '{print $1}')
    bkp_md5=$(md5sum "$backup_file" | awk '{print $1}')

    if [ "$src_md5" == "$bkp_md5" ]; then
        echo -e "${GREEN}[PASS] Incremental backup integrity verified: ${size}${NC}"
        echo "       MD5: $src_md5"
        PASSED=$((PASSED + 1))
    else
        echo -e "${RED}[FAIL] Incremental backup integrity MISMATCH: ${size}${NC}"
        echo "       Source MD5:  $src_md5"
        echo "       Backup MD5:  $bkp_md5"
        FAILED=$((FAILED + 1))
        return 1
    fi

    rm -f "$backup_file"
    return 0
}

# ==============================================================================
# multi_channel_backup: Simulate RMAN multi-channel backup
# Writes multiple backup pieces in parallel, then verifies each.
# ==============================================================================
multi_channel_backup() {
    local src_file="$1"
    local size="$2"
    local channels=4
    local file_size_bytes
    file_size_bytes=$(stat -c%s "$src_file")
    local piece_size=$((file_size_bytes / channels))

    echo -e "${CYAN}------------------------------------------------------------${NC}"
    echo -e "${CYAN}Multi-Channel Backup: size=${size} channels=${channels}${NC}"
    echo -e "${CYAN}------------------------------------------------------------${NC}"

    # Split the source into pieces and write them in parallel
    local pids=()
    for i in $(seq 0 $((channels - 1))); do
        local piece_file="$BACKUP_DIR/piece_${size}_ch${i}.bkp"
        local skip_blocks=$((i * piece_size / 1048576))
        local count_blocks=$((piece_size / 1048576))
        # Last channel gets any remaining bytes
        if [ $i -eq $((channels - 1)) ]; then
            count_blocks=$(( (file_size_bytes - i * piece_size) / 1048576 ))
        fi
        dd if="$src_file" of="$piece_file" bs=1M skip="$skip_blocks" count="$count_blocks" conv=fsync status=none &
        pids+=($!)
    done

    # Wait for all channels to complete
    local write_failed=0
    for pid in "${pids[@]}"; do
        wait "$pid" || write_failed=1
    done

    if [ $write_failed -eq 1 ]; then
        echo -e "${RED}[FAIL] Multi-channel backup write failed: ${size}${NC}"
        FAILED=$((FAILED + 1))
        return 1
    fi

    # Drop page cache
    sudo sh -c 'echo 3 > /proc/sys/vm/drop_caches' 2>/dev/null

    # Reassemble and verify
    local reassembled="$BACKUP_DIR/reassembled_${size}.bkp"
    > "$reassembled"
    for i in $(seq 0 $((channels - 1))); do
        cat "$BACKUP_DIR/piece_${size}_ch${i}.bkp" >> "$reassembled"
    done

    local src_md5
    local asm_md5
    src_md5=$(md5sum "$src_file" | awk '{print $1}')
    asm_md5=$(md5sum "$reassembled" | awk '{print $1}')

    if [ "$src_md5" == "$asm_md5" ]; then
        echo -e "${GREEN}[PASS] Multi-channel backup integrity verified: ${size}${NC}"
        echo "       MD5: $src_md5"
        PASSED=$((PASSED + 1))
    else
        echo -e "${RED}[FAIL] Multi-channel backup integrity MISMATCH: ${size}${NC}"
        echo "       Source MD5:      $src_md5"
        echo "       Reassembled MD5: $asm_md5"
        FAILED=$((FAILED + 1))
        return 1
    fi

    rm -f "$BACKUP_DIR/piece_${size}_"*.bkp "$reassembled"
    return 0
}

# ==============================================================================
# Main test execution
# ==============================================================================
echo "============================================================"
echo "Starting RMAN Backup Simulation on $MOUNT_POINT"
echo "Database file sizes: ${SIZES}"
echo "============================================================"

for SIZE in "${SIZE_ARRAY[@]}"; do
    echo ""
    echo "============================================================"
    echo "Testing with database file size: ${SIZE}"
    echo "============================================================"

    # Generate source datafile
    SRC_FILE=$(generate_datafile "$SIZE")
    if [ $? -ne 0 ]; then
        FAILED=$((FAILED + 1))
        continue
    fi

    # Test 1: Full backup with 256K block size (typical RMAN default)
    full_backup "$SRC_FILE" "$SIZE" "256K"

    # Test 2: Full backup with 1M block size (RMAN MAXPIECESIZE-style large writes)
    full_backup "$SRC_FILE" "$SIZE" "1M"

    # Test 3: Incremental backup simulation
    incremental_backup "$SRC_FILE" "$SIZE"

    # Test 4: Multi-channel parallel backup (skip for smallest size)
    if [ "$SIZE" != "10M" ]; then
        multi_channel_backup "$SRC_FILE" "$SIZE"
    fi

    # Clean up source file after testing this size
    rm -f "$SRC_FILE"
done

echo ""
echo "============================================================"
echo "RMAN Backup Simulation Complete"
echo "  Passed: $PASSED"
echo "  Failed: $FAILED"
echo "============================================================"

if [ $FAILED -gt 0 ]; then
    echo -e "${RED}Some tests FAILED!${NC}"
    exit 1
fi

echo -e "${GREEN}All RMAN backup simulation tests passed!${NC}"
exit 0
