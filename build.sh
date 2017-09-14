#!/bin/bash
BLOBFS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

##install cmake, libcurl and libfuse first
## sudo apt-get install libfuse-dev cmake libcurl4-openssl-dev -y
cd $BLOBFS_DIR/azure-storage-cpp-light/src
mkdir build
cd build
## Use cmake3 if it's available.  If not, then fallback to the default "cmake", which will hopefully be of a version > 3.5.  Otherwise, fail.
cmake3 -DCMAKE_BUILD_TYPE=Release ..
if [ $? -ne 0 ]
then
    cmake -DCMAKE_BUILD_TYPE=Release ..
fi 
if [ $? -ne 0 ]
then
	ERRORCODE=$?
	echo "cmake failed.  Please ensure that cmake version 3.5 or greater is installed and available."
	exit ERRORCODE
fi
make
cd $BLOBFS_DIR/blobfuse
make
