#!/bin/bash

if [ "$1" == "fuse2" ]
then
    # Build blobfuse2 with fuse2
    rm -rf blobfuse2
    rm -rf azure-storage-fuse
    go build -tags fuse2 -o blobfuse2
elif [ "$1" == "health" ]
then
    # Build Health Monitor binary
    rm -rf healthmon
    go build -o healthmon ./tools/health-monitor/
else
    # Build blobfuse2 with fuse3
    rm -rf blobfuse2
    rm -rf azure-storage-fuse
    go build -o blobfuse2
fi