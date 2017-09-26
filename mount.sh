#!/bin/bash
BLOBFS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd $BLOBFS_DIR/build
./blobfuse $1 --tmpPath=/mnt/blobfusetmp -o big_writes -o max_read=131072 -o max_write=131072 -o attr_timeout=240 -o fsname=blobfuse -o kernel_cache -o entry_timeout=240 -o negative_timeout=120 --configFile=../connection.cfg
