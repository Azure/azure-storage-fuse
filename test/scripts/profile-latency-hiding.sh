#!/bin/bash
# Needs to be run with sudo
# This scripts adds a computation time after reading a block, this simulates real training scenarios.
# Due to prefetch, we should hide actual network latency due to prefetching in the background.

set -euxo pipefail

BLOBFUSE2_DIR="/tmp/mntpoint"
RAMDISK_DIR="/tmp/cache"

# CLean up cache path
rm -rf $RAMDISK_DIR/*

for i in `seq 0 40`; do
    fusermount -u $BLOBFUSE2_DIR
    
    # note: blobfuse2 install see
    # https://learn.microsoft.com/en-us/azure/storage/blobs/blobfuse2-how-to-deploy#configure-the-microsoft-package-repository

    # mounts
    ./blobfuse2 mount /tmp/mntpoint --config-file=./config.yml -o ro

    # clear caches to be sure
    sync
    echo 3 > /proc/sys/vm/drop_caches

    # sometimes blobfuse terminates without error but reports no files in mount dir for a second
    sleep 1
    
    t="`awk \"BEGIN {print ($i*0.5)}\"`"
    julia latency_hiding.jl $t
done