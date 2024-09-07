#!/bin/bash

mkdir -p build && cd build
cmake -DCMAKE_BUILD_TYPE=Debug ..
#cmake -DCMAKE_BUILD_TYPE=Release ..
#cmake -DCMAKE_BUILD_TYPE=Debug -DENABLE_NO_FUSE=ON ..
make
