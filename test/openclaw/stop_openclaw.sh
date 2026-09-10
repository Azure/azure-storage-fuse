#!/usr/bin/env bash
#
# stop_openclaw.sh
# ---------------------------------------------------------------------------
# Gracefully stops OpenClaw and unmounts the BlobFuse2 mount.
#
# What this script does:
#   1. Shows what processes are currently holding the mount (diagnostic)
#   2. Stops the OpenClaw gateway gracefully
#   3. Unmounts all BlobFuse2 mounts
#
# Usage:
#   chmod +x stop_openclaw.sh
#   ./stop_openclaw.sh
# ---------------------------------------------------------------------------

# Fail fast and loudly:
#   -e : exit on any command error
#   -u : treat unset variables as errors
#   -o pipefail : a pipeline fails if any component fails
set -euo pipefail

# Path to the BlobFuse2 mount point used by OpenClaw.
MOUNT_POINT="/home/sourav/.openclaw"

# Small helper for consistent, visible status messages.
log() {
    printf '\n\033[1;34m==>\033[0m %s\n' "$1"
}

# ---------------------------------------------------------------------------
# 1. Confirm what's holding the mount (diagnostic only)
# ---------------------------------------------------------------------------
# These commands help identify any process still using the mount, which would
# otherwise cause the unmount to fail with "target is busy".
# They are informational, so we don't want the script to abort if they return
# a non-zero status (e.g. when nothing is holding the mount).
log "Checking which processes are holding the mount at ${MOUNT_POINT}..."
# fuser -mv "${MOUNT_POINT}" || true
# Alternative (lists open files under the mount directory):
lsof +D "${MOUNT_POINT}" || true

# ---------------------------------------------------------------------------
# 2. Stop the OpenClaw gateway gracefully
# ---------------------------------------------------------------------------
log "Stopping the OpenClaw gateway..."
openclaw gateway stop

# ---------------------------------------------------------------------------
# 3. Unmount BlobFuse2
# ---------------------------------------------------------------------------
log "Unmounting all BlobFuse2 mounts..."
/home/sourav/azure-storage-fuse/blobfuse2 unmount all

log "OpenClaw stopped and BlobFuse2 unmounted successfully."
