#!/bin/bash
# simulate_fuse_hidden.sh
# Usage: ./simulate_fuse_hidden.sh <mount-point>

MOUNT_POINT="${1:?Usage: $0 <mount-point>}"
TEST_FILE="$MOUNT_POINT/test_$$.txt"

echo "[1] Creating test file: $TEST_FILE"
echo "fuse_hidden test content" > "$TEST_FILE"

echo "[2] Opening fd and unlinking the file..."
python3 - "$TEST_FILE" <<'PYEOF' &
import sys, os, signal

path = sys.argv[1]
fd = os.open(path, os.O_RDWR)
print(f"    fd={fd} opened", flush=True)

os.unlink(path)
print(f"    File unlinked — .fuse_hidden* should now exist in mount dir", flush=True)

signal.signal(signal.SIGTERM, lambda s, f: None)
signal.pause()

os.close(fd)
print(f"    fd closed — .fuse_hidden* should now be deleted", flush=True)
PYEOF

BGPID=$!
sleep 1

echo ""
echo "[3] Listing .fuse_hidden* in mount directory:"
ls -la "$MOUNT_POINT"/.fuse_hidden* 2>/dev/null || echo "    (none visible — libfuse hides them from readdir)"

echo ""
read -rp ">>> Check your blob container now. Press ENTER when done to close fd and clean up..."

kill -TERM "$BGPID" 2>/dev/null
wait "$BGPID" 2>/dev/null || true
sleep 1

echo ""
echo "[4] Listing .fuse_hidden* after fd close:"
ls -la "$MOUNT_POINT"/.fuse_hidden* 2>/dev/null || echo "    (none — cleaned up)"
