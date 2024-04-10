# Blobfuse2 - A Microsoft supported Azure Storage FUSE driver
## About
Blobfuse2 is an open source project developed to provide a virtual filesystem backed by the Azure Storage. It uses the libfuse open source library (fuse3) to communicate with the Linux FUSE kernel module, and implements the filesystem operations using the Azure Storage REST APIs.

## Performance Benchmarking
To keep track of performance regression introduced by any commit in the main branch, we run a continuous benchmark test suite. This test suite is executed periodically and for each commit done to main branch. Suite uses `fio`, a industry standard storage benchmarking tool, and few custom applications to perform various tests. Results are then published in our gh-pages in this repository for a visual representaiton.

## Setup
### VM
All tests are performed on `Standard D96ds_v5` (96 vcpus, 384 GiB memory) Azure VM running in `eastus2` region. Specifications of this VM is listed [here](https://learn.microsoft.com/en-us/azure/virtual-machines/ddv5-ddsv5-series#ddsv5-series). 

### Storage
A `Permium Blob Storage Account` in `eastus2` region was used to conduct all tests. HNS was disabled on this account 

### Blobfuse2 configuration
Blobfuse2 is configured with block-cache for all tests. Other than `Large Threads` case persistance of blocks on the disk was disabled. Configuration file used in this test is available [here](https://github.com/Azure/azure-storage-fuse/blob/vibhansa/perftestrunner/testdata/config/azure_block_bench.yaml) for reference.


## Tests and Results
Master test script that simulates this benchmarking test suite is located [here](https://github.com/Azure/azure-storage-fuse/tree/vibhansa/perftestrunner/perf_testing/scripts/fio_bench.sh). To execute a specific test case download the script and execute below command:
```
    fio_bench.sh <mount-path> <test-name>
```
- Script expects Blobfuse2 is already install on the system.
- Script expects the config file is present in current directory with name `config.yaml`
- Install `fio` and `jq` before you execute the script
- Allowed `test-name` are: read / write / create / list / app


### Write
In this test `fio` command is used to run various write workflows. As part of the test `bandwidth` and `latency` are measured. Each test is run for 30 seconds and average of 3 such iteration is taken. Both `bandwidth` and `latency` are taken from `fio` output directly and projected in the charts. To simulate different write work-flows following cases are performed:
```
    - Sequential write on a 100G file
    - Sequential write on a 100G file with direct-io
    - Sequential write by 4 parallel threads on 4 different files of 100G size.
    - Sequential write by 16 parallel threads on 16 different files of 100G size.
```
All fio config files used during these tests are located [here](https://github.com/Azure/azure-storage-fuse/tree/vibhansa/perftestrunner/perf_testing/config/write).

Results for `bandwidth` of these tests are located [here](https://azure.github.io/azure-storage-fuse/bandwidth/write/).

Results for `latency` of these tests are located [here](https://azure.github.io/azure-storage-fuse/latency/write/).

### Read
In this test `fio` command is used to run various read workflows. As part of the test `bandwidth` and `latency` are measured. Each test is run for 30 seconds and average of 3 such iteration is taken. Both `bandwidth` and `latency` are taken from `fio` output directly and projected in the charts. To simulate different read work-flows following cases are performed:
```
    - Sequential read on a 100G file
    - Random read on a 100G file
    - Sequential read on a small 5M file
    - Random read on a small 5M file
    - Sequential read on a 100G file with direct-io
    - Random read on a 100G file with direct-io
    - Sequential read on a 100G file by 4 parallel threads
    - Sequential read on a 100G file by 16 parallel threads
    - Random read on a 100G file by 4 parallel threads
```
All fio config files used during these tests are located [here](https://github.com/Azure/azure-storage-fuse/tree/vibhansa/perftestrunner/perf_testing/config/read).

Results for `bandwidth` of these tests are located [here](https://azure.github.io/azure-storage-fuse/bandwidth/read/).

Results for `latency` of these tests are located [here](https://azure.github.io/azure-storage-fuse/latency/read/).


### High Threads
In this test `fio` command is used to run various read/write workflows with high number of parallel threads. As part of the test `bandwidth` and `latency` are measured. Each test is run for 60 seconds and average of 3 such iteration is taken. Both `bandwidth` and `latency` are taken from `fio` output directly and projected in the charts. To simulate different read/write work-flows following cases are performed:
```
    - Sequential write of 1G file each by 112 thread
    - Sequential read of 1G file each by 128 thread
    - Sequential read of 1G file each by 128 thread with direct-io
```
All fio config files used during these tests are located [here](https://github.com/Azure/azure-storage-fuse/tree/vibhansa/perftestrunner/perf_testing/config/high_threads).

Results for `bandwidth` of these tests are located [here](https://azure.github.io/azure-storage-fuse/bandwidth/highlyparallel/).

Results for `latency` of these tests are located [here](https://azure.github.io/azure-storage-fuse/latency/highlyparallel/).

### Create
In this test `fio` command is used to create large number of small files in parallel. As part of the test `bandwidth` and `latency` are measured. Following cases are performed as part of this test:
```
    - Create 1000 files of 1M each in 10 parallel threads
    - Create 1000 files of 1M each in 100 parallel threads
    - Create 100,000 files of 1K each in 20 parallel threads
```
All fio config files used during these tests are located [here](https://github.com/Azure/azure-storage-fuse/tree/vibhansa/perftestrunner/perf_testing/config/create).

Results for `bandwidth` of these tests are located [here](https://azure.github.io/azure-storage-fuse/bandwidth/create/).

Results for `latency` of these tests are located [here](https://azure.github.io/azure-storage-fuse/latency/create/).

### List
In this test standard linux `ls -U --color=never` command is used to list all files on the mount path. Graphs are plotted based on total time taken by this operation. Post listing this test case also execute a delete operation to delete all files on the container and measures total time take to delete these files.
For the benchmarking purpose this test is executed after `Create` tests so that storage container has 100K+ files.

Results for `time taken` by this test are located [here](https://azure.github.io/azure-storage-fuse/time/list/).

### Application Test
We have observed that `fio` and `dd` commands have certain overhead of their own and they are not able to utilize our fuse solution to its full potential. With such tools you will observe tool itself consuming 100% CPU and performance of Blobfuse2 being blocked by the tool itself. To overcome this we created custom python applications to simulate sequential red/write of a large file. 

These applications are located [here](https://github.com/Azure/azure-storage-fuse/tree/vibhansa/perftestrunner/perf_testing/scripts/). Read/Write application is executed once and based on the time taken to read/write given amount of data bandwidth is computed. Final results are published [here](https://azure.github.io/azure-storage-fuse/bandwidth/app/).


