# Blobfuse2 - A Microsoft supported Azure Storage FUSE driver
## About
Blobfuse2 is an open source project developed to provide a virtual filesystem backed by the Azure Storage. It uses the libfuse open source library (fuse3) to communicate with the Linux FUSE kernel module, and implements the filesystem operations using the Azure Storage REST APIs.

## Performance Benchmarking
To keep track of performance regression introduced by any commit in the main branch, we run a continuous benchmark test suite. This test suite is executed periodically and for each commit done to main branch. Suite uses `fio`, a industry standard storage benchmarking tool, and few custom applications to perform various tests. Results are then published in our gh-pages in this repository for a visual representaiton.

## Setup
### VM
X86_64 tests are performed on `Standard D96ds_v5` (96 vcpus, 384 GiB memory) Azure VM running in `eastus2` region. ARM64 tests are performed on `Standard D96pds_v6` (96 vcpus, 384 GiB memory) Azure VM running in `eastus2` region. Specifications of this VM is listed [here](https://learn.microsoft.com/en-us/azure/virtual-machines/ddv5-ddsv5-series#ddsv5-series). 

### Storage
A `Premium Blob Storage Account` and a `Standard Blob Storage Account` in `eastus2` region were used to conduct all tests. HNS was disabled on both these accounts.

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
- Allowed `test-name` are: read / write / create / list / app / rename

Below table provides `latency/time` and `bandwidth` results for various tests on respective account types. Each test has a linked section describing details of that test case.

### X86_64 Results

| Test Name | Premium Blob Storage Account | Standard Blob Storage Account | 
| ----------- | -------------- | ----------- |
| [Create](https://azure.github.io/azure-storage-fuse/#create)    |  [Latency](https://azure.github.io/azure-storage-fuse/X86/premium/latency/create/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/X86/premium/bandwidth/create/)  |  [Latency](https://azure.github.io/azure-storage-fuse/X86/standard/latency/create/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/X86/standard/bandwidth/create/) |
| [Write](https://azure.github.io/azure-storage-fuse/#write)     |  [Latency](https://azure.github.io/azure-storage-fuse/X86/premium/latency/write/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/X86/premium/bandwidth/write/)   | [Latency](https://azure.github.io/azure-storage-fuse/X86/standard/latency/write/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/X86/standard/bandwidth/write/) | 
| [Read](https://azure.github.io/azure-storage-fuse/#read)     |  [Latency](https://azure.github.io/azure-storage-fuse/X86/premium/latency/read/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/X86/premium/bandwidth/read/)    |  [Latency](https://azure.github.io/azure-storage-fuse/X86/standard/latency/read/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/X86/standard/bandwidth/read/) | 
| [List](https://azure.github.io/azure-storage-fuse/#list)      |  [Time](https://azure.github.io/azure-storage-fuse/X86/premium/time/list/)         |  [Time](https://azure.github.io/azure-storage-fuse/X86/standard/time/list/)    | 
| [Rename](https://azure.github.io/azure-storage-fuse/#rename)    |  [Time](https://azure.github.io/azure-storage-fuse/X86/premium/time/rename/)       |  [Time](https://azure.github.io/azure-storage-fuse/X86/standard/time/rename/)   | 
| [Highly Parallel](https://azure.github.io/azure-storage-fuse/#highly-parallel) |  [Latency](https://azure.github.io/azure-storage-fuse/X86/premium/latency/highlyparallel/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/X86/premium/bandwidth/highlyparallel/)  | [Latency](https://azure.github.io/azure-storage-fuse/X86/standard/latency/highlyparallel/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/X86/standard/bandwidth/highlyparallel/) | 
| [Application](https://azure.github.io/azure-storage-fuse/#application-test)       |  [Time](https://azure.github.io/azure-storage-fuse/X86/premium/time/app/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/X86/premium/bandwidth/app/) |  [Time](https://azure.github.io/azure-storage-fuse/X86/standard/time/app/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/X86/standard/bandwidth/app/) |
| [Max Out](https://azure.github.io/azure-storage-fuse/#max-out)       |  [Time](https://azure.github.io/azure-storage-fuse/X86/premium/time/highapp/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/X86/premium/bandwidth/highapp/) |  [Time](https://azure.github.io/azure-storage-fuse/X86/standard/time/highapp/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/X86/standard/bandwidth/highapp/) |  


### ARM64 Results

| Test Name | Premium Blob Storage Account | Standard Blob Storage Account | 
| ----------- | -------------- | ----------- |
| [Create](https://azure.github.io/azure-storage-fuse/#create)    |  [Latency](https://azure.github.io/azure-storage-fuse/ARM/premium/latency/create/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/ARM/premium/bandwidth/create/)  |  [Latency](https://azure.github.io/azure-storage-fuse/ARM/standard/latency/create/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/ARM/standard/bandwidth/create/) |
| [Write](https://azure.github.io/azure-storage-fuse/#write)     |  [Latency](https://azure.github.io/azure-storage-fuse/ARM/premium/latency/write/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/ARM/premium/bandwidth/write/)   | [Latency](https://azure.github.io/azure-storage-fuse/ARM/standard/latency/write/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/ARM/standard/bandwidth/write/) | 
| [Read](https://azure.github.io/azure-storage-fuse/#read)     |  [Latency](https://azure.github.io/azure-storage-fuse/ARM/premium/latency/read/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/ARM/premium/bandwidth/read/)    |  [Latency](https://azure.github.io/azure-storage-fuse/ARM/standard/latency/read/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/ARM/standard/bandwidth/read/) | 
| [List](https://azure.github.io/azure-storage-fuse/#list)      |  [Time](https://azure.github.io/azure-storage-fuse/ARM/premium/time/list/)         |  [Time](https://azure.github.io/azure-storage-fuse/ARM/standard/time/list/)    | 
| [Rename](https://azure.github.io/azure-storage-fuse/#rename)    |  [Time](https://azure.github.io/azure-storage-fuse/ARM/premium/time/rename/)       |  [Time](https://azure.github.io/azure-storage-fuse/ARM/standard/time/rename/)   | 
| [Highly Parallel](https://azure.github.io/azure-storage-fuse/#highly-parallel) |  [Latency](https://azure.github.io/azure-storage-fuse/ARM/premium/latency/highlyparallel/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/ARM/premium/bandwidth/highlyparallel/)  | [Latency](https://azure.github.io/azure-storage-fuse/ARM/standard/latency/highlyparallel/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/ARM/standard/bandwidth/highlyparallel/) | 
| [Application](https://azure.github.io/azure-storage-fuse/#application-test)       |  [Time](https://azure.github.io/azure-storage-fuse/ARM/premium/time/app/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/ARM/premium/bandwidth/app/) |  [Time](https://azure.github.io/azure-storage-fuse/ARM/standard/time/app/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/ARM/standard/bandwidth/app/) |
| [Max Out](https://azure.github.io/azure-storage-fuse/#max-out)       |  [Time](https://azure.github.io/azure-storage-fuse/ARM/premium/time/highapp/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/ARM/premium/bandwidth/highapp/) |  [Time](https://azure.github.io/azure-storage-fuse/ARM/standard/time/highapp/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/ARM/standard/bandwidth/highapp/) |  


### Create
In this test `fio` command is used to create large number of small files in parallel. As part of the test `bandwidth` and `latency` are measured. Following cases are performed as part of this test:
```
    - Create 1000 files of 1M each in 10 parallel threads
    - Create 1000 files of 1M each in 100 parallel threads
    - Create 100,000 files of 1K each in 20 parallel threads
```
All fio config files used during these tests are located [here](https://github.com/Azure/azure-storage-fuse/tree/vibhansa/perftestrunner/perf_testing/config/create).

### Write
In this test `fio` command is used to run various write workflows. As part of the test `bandwidth` and `latency` are measured. Each test is run for 30 seconds and average of 3 such iteration is taken. Both `bandwidth` and `latency` are taken from `fio` output directly and projected in the charts. To simulate different write work-flows following cases are performed:
```
    - Sequential write on a 100G file
    - Sequential write on a 100G file with direct-io
    - Sequential write by 4 parallel threads on 4 different files of 100G size.
    - Sequential write by 16 parallel threads on 16 different files of 100G size.
```
All fio config files used during these tests are located [here](https://github.com/Azure/azure-storage-fuse/tree/vibhansa/perftestrunner/perf_testing/config/write).

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

### List
In this test standard linux `ls -U --color=never` command is used to list all files on the mount path. Graphs are plotted based on total time taken by this operation. Post listing this test case also execute a delete operation to delete all files on the container and measures total time take to delete these files.
For the benchmarking purpose this test is executed after `Create` tests so that storage container has 100K+ files.

### Rename
In this test 5000 files of 1MB size each are created using a python script and then each file is renamed using os.rename() method. Total time to rename all 5000 files is measured. 

### Highly Parallel
In this test `fio` command is used to run various read/write workflows with high number of parallel threads. As part of the test `bandwidth` and `latency` are measured. Each test is run for 60 seconds and average of 3 such iteration is taken. Both `bandwidth` and `latency` are taken from `fio` output directly and projected in the charts. To simulate different read/write work-flows following cases are performed:
```
    - Sequential write of 1G file each by 112 thread
    - Sequential read of 1G file each by 128 thread
    - Sequential read of 1G file each by 128 thread with direct-io
```
All fio config files used during these tests are located [here](https://github.com/Azure/azure-storage-fuse/tree/vibhansa/perftestrunner/perf_testing/config/high_threads).

### Application Test
We have observed that `fio` and `dd` commands have certain overhead of their own and they are not able to utilize our fuse solution to its full potential. With such tools you will observe tool itself consuming 100% CPU and performance of Blobfuse2 being blocked by the tool itself. To overcome this we created custom python applications to simulate sequential red/write of a large file. 

These applications are located [here](https://github.com/Azure/azure-storage-fuse/tree/vibhansa/perftestrunner/perf_testing/scripts/). 
Read/Write application is executed once and based on the time taken to read/write given amount of data bandwidth is computed. 

### Max Out
Objective of this test case is to observe the max throughput that Blobfuse2 can generate. To achieve this two tests are run:
```
    - Create 10 20GB files
    - Read 10 20GB files
```

In case test 10 threads are run in parallel. Each thread is bound to a CPU core so that it gets max CPU for operation. 'dd' command is used to execute the test.

For write operation, data is read from '/dev/zero' in 16MB chunks and written to file on mounted path with 'direct io'. 

For read operations, data is read from mount path in 4MB chunks and sent to '/dev/null' with 'dd' set to not report any status. 

Test Scripts are located [here](https://github.com/Azure/azure-storage-fuse/tree/vibhansa/perftestrunner/perf_testing/scripts/) with name "highspeed_".
