#!/bin/bash
BLOBFS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd $BLOBFS_DIR/build
./blobfuse $1 --tmp-path=/mnt/blobfusetmp --use-attr-cache=true -o attr_timeout=240 -o entry_timeout=240 -o negative_timeout=120 --config-file=../connection.cfg
