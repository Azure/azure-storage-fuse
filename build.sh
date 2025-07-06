#!/bin/bash

#
# For making debug builds with asserts, run as:
# RELEASE_BUILD=0 ./build.sh
#
# For making release builds with all calls to asserts stripped off, run as:
# ./build.sh
#
export RELEASE_BUILD=${RELEASE_BUILD:-1}

echo "Using Go - $(go version)"
if [ "$1" == "fuse2" ]
then
    # Build blobfuse2 with fuse2
    rm -rf blobfuse2
    rm -rf azure-storage-fuse
    go build -tags fuse2 -o blobfuse2
elif [ "$1" == "health" ]
then
    # Build Health Monitor binary
    rm -rf bfusemon
    go build -o bfusemon ./tools/health-monitor/
else
    # Build blobfuse2 with fuse3
    rm -rf blobfuse2
    rm -rf azure-storage-fuse

    if [ "$RELEASE_BUILD" == "1" ]
    then
        echo "Building blobfuse2 release build"
        (cd ./tools/assert-remover; go build remove_asserts.go)
        ./tools/assert-remover/do.sh
        go build -o blobfuse2
        ./tools/assert-remover/undo.sh
    else
        echo "Building blobfuse2 debug build"
        go build -o blobfuse2
    fi
fi
