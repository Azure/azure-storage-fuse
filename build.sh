#!/bin/bash
BLOBFS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

##install cmake, libcurl and libfuse first
## sudo apt-get install libfuse-dev cmake libcurl4-openssl-dev -y
cd $BLOBFS_DIR/azure-storage-cpp-light/src
mkdir build
cd build
cmake3 -DCMAKE_BUILD_TYPE=Release ..
make
cd $BLOBFS_DIR/blobfuse
make
