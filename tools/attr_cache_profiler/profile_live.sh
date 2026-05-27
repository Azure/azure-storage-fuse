#!/usr/bin/env bash
# Live attr_cache memory profiling via real blobfuse2.
#
# positive mode: mounts blobfuse2, stats N synthetic paths with the
#                dummy_positive_entry prefix; loopback_fs.GetAttr returns a
#                synthetic ObjAttr for each, so attr_cache caches positive entries
#                without any real files on disk.
#
# negative mode: mounts blobfuse2, stats N synthetic paths with the
#                dummy_negative_entry prefix; loopback_fs.GetAttr returns ENOENT
#                for each, so attr_cache caches a negative entry per path.
#
# Memory is measured via /proc/<pid>/status and pprof heap snapshot.
#
# Usage:
#   ./tools/attr_cache_profiler/profile_live.sh positive [N]
#   ./tools/attr_cache_profiler/profile_live.sh negative [N]
#
# Requirements:
#   - ~/mnt must exist          : mkdir -p ~/mnt
#   - /mnt/loopback must exist  : sudo mkdir -p /mnt/loopback && sudo chown $USER /mnt/loopback

set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MODE="${1:?Usage: $0 positive|negative [N]}"
N="${2:-5000000}"
MOUNT="$HOME/mnt"
LOOPBACK="/mnt/loopback"
BLOBFUSE2="$REPO_DIR/blobfuse2"
CONFIG="$REPO_DIR/config.yaml"
PPROF_PORT=6060
HEAP_OUT="/tmp/attr_cache_heap_${MODE}.pb.gz"

[[ "$MODE" == "positive" || "$MODE" == "negative" ]] || {
    echo "ERROR: mode must be 'positive' or 'negative'"
    exit 1
}

# ---------- build ----------
echo "=== Building ==="
cd "$REPO_DIR"
go build -o blobfuse2 .
go build -o /tmp/attr_cache_fuse_sweep ./tools/attr_cache_profiler/fuse_sweep/
echo "  OK"

# ---------- sanity checks ----------
[[ -d "$LOOPBACK" ]] || { echo "ERROR: $LOOPBACK not found. Run: sudo mkdir -p $LOOPBACK && sudo chown \$USER $LOOPBACK"; exit 1; }
mkdir -p "$MOUNT"

# ---------- unmount if already mounted ----------
if mountpoint -q "$MOUNT" 2>/dev/null; then
    echo "Unmounting existing $MOUNT..."
    fusermount -u "$MOUNT"
    sleep 1
fi

# ---------- create a profile config with pprof enabled ----------
PROFILE_CFG="/tmp/blobfuse2_profile.yaml"
cp "$CONFIG" "$PROFILE_CFG"
grep -q "^dynamic-profile" "$PROFILE_CFG" || \
    printf '\ndynamic-profile: true\nprofiler-port: %d\n' "$PPROF_PORT" >> "$PROFILE_CFG"

echo ""
echo "=== MODE: $MODE  N: $N ==="

# ---------- mount ----------
echo ""
echo "--- Mounting blobfuse2 ---"
"$BLOBFUSE2" mount "$MOUNT" --config-file="$PROFILE_CFG"

echo -n "  Waiting for mount..."
for i in $(seq 1 20); do
    mountpoint -q "$MOUNT" 2>/dev/null && break
    sleep 1; echo -n "."
done
echo ""
mountpoint -q "$MOUNT" || { echo "ERROR: mount failed. Check $REPO_DIR/logs/blobfuse2.log"; exit 1; }

# blobfuse2 daemonizes: parent exits, child is the real process
BF2_PID=$(pgrep -f "blobfuse2 mount" 2>/dev/null | head -1 || pgrep blobfuse2 2>/dev/null | head -1 || true)
echo "  blobfuse2 PID: ${BF2_PID:-unknown}"

# wait for pprof HTTP server
echo -n "  Waiting for pprof..."
for i in $(seq 1 10); do
    curl -sf --connect-timeout 1 "http://localhost:${PPROF_PORT}/debug/pprof/" -o /dev/null && break
    sleep 1; echo -n "."
done
echo ""

# ---------- baseline memory ----------
echo ""
echo "--- Baseline (cache empty) ---"
[[ -n "${BF2_PID:-}" ]] && grep -E "VmRSS|VmHWM" /proc/"$BF2_PID"/status 2>/dev/null || true

# ---------- populate attr_cache ----------
echo ""
if [[ "$MODE" == "positive" ]]; then
    echo "--- Populating $N positive entries via stat (dummy_positive_entry prefix) ---"
    /tmp/attr_cache_fuse_sweep -mount "$MOUNT" -mode positive -n "$N" -workers 16
else
    echo "--- Populating $N negative entries via stat (dummy_negative_entry prefix) ---"
    /tmp/attr_cache_fuse_sweep -mount "$MOUNT" -mode negative -n "$N" -workers 16
fi

# ---------- memory after population ----------
echo ""
echo "--- Memory after populating $N entries ---"
if [[ -n "${BF2_PID:-}" ]]; then
    grep -E "VmPeak|VmSize|VmRSS|VmHWM|VmData|VmSwap" /proc/"$BF2_PID"/status 2>/dev/null || echo "  (unavailable)"
fi

# ---------- pprof heap snapshot ----------
echo ""
echo "--- pprof heap snapshot ---"
if curl -sf --connect-timeout 5 --max-time 60 \
        "http://localhost:${PPROF_PORT}/debug/pprof/heap" -o "$HEAP_OUT"; then
    echo "  Saved: $HEAP_OUT"
    echo ""
    go tool pprof -top "$HEAP_OUT" 2>/dev/null | head -20 || true
    echo ""
    echo "  Interactive: go tool pprof -http=:8080 $HEAP_OUT"
else
    echo "  WARNING: could not reach pprof at localhost:${PPROF_PORT}"
fi

# ---------- summary ----------
echo ""
echo "=== SUMMARY: $MODE / $N entries ==="
if [[ -n "${BF2_PID:-}" ]]; then
    VmRSS=$(awk '/^VmRSS/{print $2}' /proc/"$BF2_PID"/status 2>/dev/null || echo 0)
    VmHWM=$(awk '/^VmHWM/{print $2}' /proc/"$BF2_PID"/status 2>/dev/null || echo 0)
    printf "  VmRSS (current) : %d kB  (%.2f GB)\n" "$VmRSS" "$(echo "scale=2; $VmRSS/1048576" | bc)"
    printf "  VmHWM (peak)    : %d kB  (%.2f GB)\n" "$VmHWM" "$(echo "scale=2; $VmHWM/1048576" | bc)"
fi
echo "  Heap profile    : $HEAP_OUT"
echo ""
echo "blobfuse2 still running (PID ${BF2_PID:-?})"
echo "  Unmount : fusermount -u $MOUNT"
