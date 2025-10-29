#!/bin/bash

if [ ! -e "./blobfuse2" ]; then
    echo "Error: tools/run_blobfuse2.sh must be run from the directory where blobfuse2 binary is located."
    exit 1
fi

#
# When run with distributed cache, blobfuse2 can benefit from NUMA binding to improve performance.
# Use this wrapper script in place of running blobfuse2 directly.
# It also prioritizes n/w IO scheduling for metadata operations so that they are not delayed by heavy
# data operations.
#
tools/prioritize-metadata-ios.sh set
numactl --cpunodebind=0 --membind=0 ./blobfuse2 "$@"
tools/prioritize-metadata-ios.sh unset

