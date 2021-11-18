#!/usr/bin/env bash
BLOBFS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

## Use "export INCLUDE_TESTS=1" to enable building tests

## Build the cpplite lib first
#echo "Building the cpplite lib"
if [ "$1" = "debug" ]
then
#rm -rf cpplite/build.release
#rm -rf build/blobfuse
mkdir cpplite/build.release
cd cpplite/build.release
echo "Building cpplite in Debug mode"
cmake .. -DCMAKE_BUILD_TYPE=Debug -DBUILD_ADLS=ON -DUSE_OPENSSL=OFF
else
mkdir cpplite/build.release
cd cpplite/build.release
cmake .. -DCMAKE_BUILD_TYPE=Release -DBUILD_ADLS=ON -DUSE_OPENSSL=OFF
fi

cmake --build .
status=$?

if test $status -eq 0
then
	echo "************************ CPPLite build Successful ***************************** "
else
	echo "************************ CPPLite build Failed ***************************** "
	exit $status
fi
cd -

## install pkg-config, cmake, libcurl and libfuse first
## For example, on ubuntu - sudo apt-get install pkg-config libfuse-dev cmake libcurl4-openssl-dev -y
## on RHEL/CentOS sudo dnf -y install pkgconfig fuse-devel cmake curl-devel gcc gcc-c++ make gnutls-devel libuuid-devel boost-devel libgcrypt-devel rpm-build
mkdir build
cd build

# Copy the cpplite lib here
#cp ../cpplite/build.release/libazure*.a ./ 

if [ "$1" = "debug" ]
then
	cmake_args='-DCMAKE_BUILD_TYPE=Debug ..'
	if [ -n "${INCLUDE_TESTS}" ]; then
		cmake_args='-DCMAKE_BUILD_TYPE=Debug -DINCLUDE_TESTS=1 ..'
	fi
else
	cmake_args='-DCMAKE_BUILD_TYPE=RelWithDebInfo ..'
	if [ -n "${INCLUDE_TESTS}" ]; then
		cmake_args='-DCMAKE_BUILD_TYPE=RelWithDebInfo -DINCLUDE_TESTS=1 ..'
	fi
fi

kernel_ver_str=`uname -r | cut -d "." -f1,2`
kernel_ver=$(bc -l <<< "$kernel_ver_str")
echo "######################## KERNEL VERSION $kernel_ver ###################################"
if [ "$1" = "debug" ]
then
	if [ 1 -eq "$(echo "${kernel_ver} < 4.16" | bc)" ]
	then
		echo "Kernel version is lower then 4.16"
		cmake_args="-DINCLUDE_EXTRALIB=1 $cmake_args"
	else
		echo "Kernel version is bigger then 4.16"
	fi
fi

if [ -n "${AZS_FUSE3}" ]; then
   cmake_args="-DAZS_FUSE3=1 $cmake_args"
fi

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
