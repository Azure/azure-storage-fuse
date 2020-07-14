# Azure Storage C++ Client Library (Lite)

## About

The Azure Storage Client Library (Lite) for C++ allows you to build applications against Microsoft Azure Storage's blob service. This is a minimum dependency version that provide basic object storage. For an overview of Azure Storage, see [Introduction to Microsoft Azure Storage](http://azure.microsoft.com/en-us/documentation/articles/storage-introduction/).
If you want to use other services of Azure Storage, or a more comprehensive functionality of Blob service, please see [Azure Storage C++ Client Library](https://github.com/azure/azure-storage-cpp).

## Features
The full supported Azure Storage API can be found in the following list, please be aware that only part of the functionality of some APIs are supported:
- [List Containers](https://docs.microsoft.com/en-us/rest/api/storageservices/list-containers2).
- [Create Container](https://docs.microsoft.com/en-us/rest/api/storageservices/create-container).
- [Get Container Properties](https://docs.microsoft.com/en-us/rest/api/storageservices/get-container-properties).
- [Delete Container](https://docs.microsoft.com/en-us/rest/api/storageservices/delete-container).
- [List Blobs](https://docs.microsoft.com/en-us/rest/api/storageservices/list-blobs).
- [Put Blob](https://docs.microsoft.com/en-us/rest/api/storageservices/put-blob).
- [Get Blob](https://docs.microsoft.com/en-us/rest/api/storageservices/get-blob).
- [Get Blob Properties](https://docs.microsoft.com/en-us/rest/api/storageservices/get-blob-properties).
- [Delete Blob](https://docs.microsoft.com/en-us/rest/api/storageservices/delete-blob).
- [Copy Blob](https://docs.microsoft.com/en-us/rest/api/storageservices/copy-blob).
- [Put Block](https://docs.microsoft.com/en-us/rest/api/storageservices/put-block).
- [Put Block List](https://docs.microsoft.com/en-us/rest/api/storageservices/put-block-list).
- [Get Block List](https://docs.microsoft.com/en-us/rest/api/storageservices/get-block-list).
- [Put Page](https://docs.microsoft.com/en-us/rest/api/storageservices/put-page).
- [Get Page Ranges](https://docs.microsoft.com/en-us/rest/api/storageservices/get-page-ranges).
- [Append Block](https://docs.microsoft.com/en-us/rest/api/storageservices/append-block).

## Installation
### Supported Platforms
Please be aware that below platforms are tested and verified, but other platforms beyond this list can be working with some modification on the build steps resolving the dependencies. Feel free to test them out and contribute back.
  - Ubuntu 16.04 x86_64.
  - CentOS 6 x86_64.
  - CentOS 7 x86_64.
  - macOS Mojave version 10.14.6 x86_64.
  - Windows 10 with Visual Studio 2017, x86 or x64.

### Build this library on Linux or macOS
Project dependencies:
  - GnuTLS (or OpenSSL v1.0.1)
  - libcurl v7.35.0
  - CMake v3.6
  - GNU C++ or Apple clang with C++11 support
  - libuuid 2.13.1

#### Clone the latest code from this repository:
```bash
git clone https://github.com/azure/azure-storage-cpplite.git
```

#### Install the dependencies, e.g. on Ubuntu:
```bash
apt install libssl-dev libcurl4-openssl-dev cmake g++ uuid-dev
```
Or, on CentOS:
```bash
yum install openssl-devel libcurl-devel cmake3 gcc-c++ libuuid-devel
```
Or, on macOS:
```bash
brew install openssl cmake
```
Please be aware that CentOS 6 comes with GCC version 4.4.7, which does not meet the requirement of this SDK. In order to use this SDK, [devtoolset](http://linux.web.cern.ch/linux/devtoolset/#install) needs to be installed properly.
Please be aware that on some Linux distributions, pkg-config is not properly installed that would result in CMake not behaving as expected. Installing pkg-config or updating it will eliminate the potential issue. The tested version of pkg-config is 0.27.1.

#### Build and install azure-storage-cpplite:
```bash
cd azure-storage-cpplite
mkdir build.release
cd build.release
cmake .. -DCMAKE_BUILD_TYPE=Release
# Just build
cmake --build .
# Or build and install
cmake --build . --target install
```
#### Use GnuTLS instead of OpenSSL:
Alternatively, you can use GnuTLS instead of OpenSSL. Simply install GnuTLS and add the argument `-DUSE_OPENSSL=OFF` during CMake configure.

### Build this library on Windows
Project dependencies:
  - OpenSSL v1.0.2
  - libcurl v7.60.0
  - CMake v3.6
  - Visual Studio 2017

#### Clone the latest code from this repository:
```bash
git clone https://github.com/azure/azure-storage-cpplite.git
```

#### Prepare and install the dependencies.
There are two major dependencies on Windows: libcurl and OpenSSL. For the best development experience, we recommend that developers use [vcpkg](https://github.com/microsoft/vcpkg) to install dependencies. You can also install your own choice of pre-built binaries.
```BatchFile
vcpkg install curl openssl
```

#### Build azure-storage-cpplite using CMake in command line
```bash
cd azure-storage-cpplite
mkdir build.release
cd build.release
cmake .. -DCMAKE_TOOLCHAIN_FILE=<vcpkg path>/scripts/buildsystems/vcpkg.cmake
cmake --build . --config Release
```

### Specifying customized OpenSSL or libcurl root folder
Default OpenSSL or libcurl root directory may not be applicable for all users. In that case, following parameters can be used to specify the preferred path:
`-DCURL_INCLUDE_DIR=<libcurl's include directory>`,
`-DCURL_LIBRARY=<libcurl's library path>`,
`-DOPENSSL_ROOT_DIR=<OpenSSL's root directory>`.

This applies to both Windows and Unix.

### Other build options
There are some advanced options to config the build when using CMake commands:

- `-DBUILD_SHARED_LIBS=` : specify `ON` or `OFF` to control if shared library or static library should be built.
- `-DBUILD_TESTS=` : specify `ON` or `OFF` to control if tests should be built.
- `-DBUILD_SAMPLES=` : specify `ON` or `OFF` to control if samples should be built.
- `-A ` : specify `Win32` or `x64` to config the generator platform type on Windows.
- `-DCMAKE_BUILD_TYPE=` : specify `Debug` or `Release` to config the build type on Linux or macOS.
- `--config `: specify `Debug` or `Release` to config the build type on Windows.

## Usage
Simply include the header files after installing the library, everything is good to go. For a more comprehensive sample, please see [sample](https://github.com/azure/azure-storage-cpplite/blob/master/sample/sample.cpp).
To build the sample, add `-DBUILD_SAMPLES=ON` when building the repository.
```C++
#include "storage_credential.h"
#include "storage_account.h"
#include "blob/blob_client.h"

// Your settings
std::string account_name = "YOUR_ACCOUNT_NAME";
std::string account_key = "YOUR_ACCOUNT_KEY";
bool use_https = true;
std::string blob_endpoint = "CUSTOMIZED_BLOB_ENDPOINT";
int connection_count = 2;

// Setup the client
azure::storage_lite::shared_key_credential credential(account_name, account_key);
azure::storage_lite::storage_account(account_name, credential, use_https, blob_endpoint);
azure::storage_lite::blob_client client(storage_account, connection_count);

// Start using
auto outcome = client.create_container("YOUR_CONTAINER_NAME").get();
```

## License
This project is licensed under MIT.
 
## Contributing
This project welcomes contributions and suggestions.  Most contributions require you to agree to a
Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us
the rights to use your contribution. For details, visit https://cla.microsoft.com.

When you submit a pull request, a CLA-bot will automatically determine whether you need to provide
a CLA and decorate the PR appropriately (e.g., label, comment). Simply follow the instructions
provided by the bot. You will only need to do this once across all repositories using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or
contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.

When contributing to this client library, there are following ground rules:
1. All source code change should be clearly addressing issues or adding new features, and should be covered with test.
2. Coding style should match with the existing code.
3. Any contribution should not degrade performance or complex functionality.
4. Introducing new dependency should be done with much great caution. Any new dependency should introduce significant performance improvement or unblock critical user scenario.

### Build Test
Install Catch2 via package manager, or download [Catch2 single header version](https://raw.githubusercontent.com/catchorg/Catch2/master/single_include/catch2/catch.hpp) and specify the location with `-DCATCH2_INCLUDE_DIR=<catch2 path>` when build.

Add `-DBUILD_TESTS=ON` when building the repository.

Please modify the [connection string here](https://github.com/azure/azure-storage-cpplite/blob/master/test/test_base.h#L19) to successfully run the tests. All the tests use standard Azure Storage account.

**Please note that in order to run test, a minimum version of g++ 5.1 is required on Linux, and Visual Studio 2017 is required on Windows.**
