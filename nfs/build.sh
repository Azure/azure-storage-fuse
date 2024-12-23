#!/bin/bash

export VCPKG_ROOT=/home/pragya/azure-storage-fuse/nfs/extern/vcpkg
mkdir -p build && cd build
#cmake -DCMAKE_BUILD_TYPE=Debug -DENABLE_TCMALLOC=OFF ..
#cmake -DCMAKE_BUILD_TYPE=Release ..
#cmake -DCMAKE_BUILD_TYPE=Debug -DENABLE_NO_FUSE=ON ..
cmake -DCMAKE_BUILD_TYPE=Debug -DENABLE_TCMALLOC=OFF -DCMAKE_TOOLCHAIN_FILE=$VCPKG_ROOT/scripts/buildsystems/vcpkg.cmake ..
cmake --build .
make
