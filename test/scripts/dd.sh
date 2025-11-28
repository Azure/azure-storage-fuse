#!/bin/bash

# Usage: ./test_fuse.sh /path/to/fuse/mountpoint
MOUNT_POINT="$1"

if [ -z "$MOUNT_POINT" ]; then
    echo "Error: Please provide the FUSE mount point."
    echo "Usage: $0 /path/to/mountpoint"
    exit 1
fi

# Ensure the mount point exists
if [ ! -d "$MOUNT_POINT" ]; then
    echo "Error: Directory $MOUNT_POINT does not exist."
    exit 1
fi

# Configuration
TEST_FILENAME="integrity_test.dat"
SOURCE_FILE="/tmp/fuse_test_source.tmp"
READ_BACK_FILE="/tmp/fuse_test_readback.tmp"
FILE_SIZE_MB=100 # Size of the test file in MB (needs to be larger than the largest block size)
BLOCK_SIZES=("128K" "1M" "10M")

echo "========================================================"
echo "Starting FUSE Filesystem Integrity Test Suite"
echo "Mount Point: $MOUNT_POINT"
echo "File Size:   ${FILE_SIZE_MB}MB"
echo "========================================================"

# Generate a random source file once to use for all tests
echo "Generating random source data..."
dd if=/dev/urandom of="$SOURCE_FILE" bs=1M count="$FILE_SIZE_MB" status=none
SOURCE_CHECKSUM=$(md5sum "$SOURCE_FILE" | awk '{print $1}')
echo "Source MD5: $SOURCE_CHECKSUM"
echo ""

# Loop through defined block sizes
for BS in "${BLOCK_SIZES[@]}"; do
    echo "--------------------------------------------------------"
    echo "TESTING BLOCK SIZE: $BS"
    echo "--------------------------------------------------------"

    FUSE_FILE="$MOUNT_POINT/$TEST_FILENAME"

    # 1. WRITE TEST
    echo "[WRITE] Writing to FUSE fs with bs=$BS..."
    # conv=fsync ensures data is physically written before dd exits
    if dd if="$SOURCE_FILE" of="$FUSE_FILE" bs="$BS" conv=fsync status=none; then
        echo "   -> Write successful."
    else
        echo "   -> Write FAILED."
        exit 1
    fi

    # Optional: Clear pagecache to force read from the FUSE fs, not RAM
    # (Requires sudo; uncomment if you have permissions and want strict testing)
    sudo sh -c 'echo 3 > /proc/sys/vm/drop_caches'

    # 2. READ TEST
    echo "[READ]  Reading back from FUSE fs with bs=$BS..."
    if dd if="$FUSE_FILE" of="$READ_BACK_FILE" bs="$BS" status=none; then
        echo "   -> Read successful."
    else
        echo "   -> Read FAILED."
        exit 1
    fi

    # 3. INTEGRITY CHECK
    echo "[CHECK] Verifying data integrity..."
    READ_CHECKSUM=$(md5sum "$READ_BACK_FILE" | awk '{print $1}')

    if [ "$SOURCE_CHECKSUM" == "$READ_CHECKSUM" ]; then
        echo "   -> PASS: Checksums match ($READ_CHECKSUM)"
    else
        echo "   -> FAIL: Checksum mismatch!"
        echo "      Expected: $SOURCE_CHECKSUM"
        echo "      Got:      $READ_CHECKSUM"
        # Clean up before exiting on failure
        rm -f "$SOURCE_FILE" "$READ_BACK_FILE"
        exit 1
    fi

    # Clean up the file on the FUSE fs for the next iteration
    rm -f "$FUSE_FILE"
    echo ""
done

# Final Cleanup
rm -f "$SOURCE_FILE" "$READ_BACK_FILE"

echo "========================================================"
echo "All tests passed successfully!"
echo "========================================================"
