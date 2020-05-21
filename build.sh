#!/usr/bin/env bash
BLOBFS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

## Use "export INCLUDE_TESTS=1" to enable building tests

cmake_args='-DCMAKE_BUILD_TYPE=RelWithDebInfo ..'
if [ -n "${INCLUDE_TESTS}" ]; then
    cmake_args='-DCMAKE_BUILD_TYPE=RelWithDebInfo -DINCLUDE_TESTS=1 ..'
fi

## Build the cpplite lib first
echo "Building the cpplite lib"
mkdir cpplite/build.release
cd cpplite/build.release
cmake .. -DCMAKE_BUILD_TYPE=Release
cmake --build .
cd -

## install pkg-config, cmake, libcurl and libfuse first
## For example, on ubuntu - sudo apt-get install pkg-config libfuse-dev cmake libcurl4-openssl-dev -y
mkdir build
cd build

# Copy the cpplite lib here
cp ../cpplite/build.release/libazure-storage-lite.a ./ 

## Use cmake3 if it's available.  If not, then fallback to the default "cmake".  Otherwise, fail.
cmake3 $cmake_args
if [ $? -ne 0 ]
then
    cmake $cmake_args
fi 
if [ $? -ne 0 ]
then
	ERRORCODE=$?
	echo "cmake failed.  Please ensure that cmake version 3.5 or greater is installed and available."
	exit $ERRORCODE
fi
make
