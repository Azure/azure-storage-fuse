#!/bin/bash
echo "Using Go - $(go version)"

# FIPS compliance: blobfuse2 ships in the Linux package feed that AKS Blob
# CSI consumes, so the binary must be built with the Microsoft Go toolchain
# (systemcrypto GOEXPERIMENT) and CGO enabled. Pin the toolchain to the
# locally installed Go so a stray `go` directive bump cannot trigger a
# silent download of an upstream non-FIPS toolchain.
export CGO_ENABLED=1
export GOTOOLCHAIN=local

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
    go build -o blobfuse2
fi
