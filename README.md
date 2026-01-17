# Blobfuse2 - A Microsoft supported Azure Storage FUSE driver
## About
Blobfuse2 is an open source project developed to provide a virtual filesystem backed by the Azure Storage. It uses the libfuse open source library (fuse3) to communicate with the Linux FUSE kernel module, and implements the filesystem operations using the Azure Storage REST APIs.

## Performance Benchmarking
To keep track of performance regression introduced by any commit in the main branch, we run a continuous benchmark test suite. This test suite is executed periodically and for each commit done to main branch. Suite uses `fio`, a industry standard storage benchmarking tool, and few custom applications to perform various tests. Results are then published in our gh-pages in this repository for a visual representaiton.

### Benchmark Action Workflow
The benchmark automation is defined in the [`.github/workflows/benchmark.yml`](https://github.com/Azure/azure-storage-fuse/blob/main/.github/workflows/benchmark.yml) workflow file. This workflow runs on a weekly schedule (every Sunday at 4 AM UTC) and can also be triggered manually. The workflow:

- Runs on self-hosted runners with specific hardware configurations (X86 and ARM64)
- Tests against both Standard and Premium Azure Storage accounts
- Tests both file-cache and block-cache configurations
- Executes read and write benchmark tests using FIO
- Publishes results to the gh-pages branch for visualization
- Runs tests sequentially (one at a time) to avoid storage performance interference

The workflow uses a matrix strategy to test all combinations of:
- **Architectures**: X86 (Standard D96ds_v5) and ARM64 (Standard D96pds_v6)
- **Storage Types**: Standard and Premium Blob Storage accounts
- **Cache Modes**: file-cache and block-cache

## Setup
### VM
X86_64 tests are performed on `Standard D96ds_v5` (96 vcpus, 384 GiB memory) Azure VM running in `eastus2` region. Specifications of this VM can be found [here](https://learn.microsoft.com/en-us/azure/virtual-machines/sizes/general-purpose/ddv5-series?tabs=sizebasic#sizes-in-series).

ARM64 tests are performed on `Standard D96pds_v6` (96 vcpus, 384 GiB memory) Azure VM running in `eastus2` region. Specifications of this VM can be found [here](https://learn.microsoft.com/en-us/azure/virtual-machines/sizes/general-purpose/dpdsv6-series?tabs=sizebasic#sizes-in-series).

#### Disk Throughput

| VM                 | Read (MB/s)                                                                 | Write (MB/s)                                                                  |
|--------------------|-----------------------------------------------------------------------------|-------------------------------------------------------------------------------|
| Standard D96ds_v5  | [Throughput](https://azure.github.io/azure-storage-fuse/X86/disk/read/)     | [Throughput](https://azure.github.io/azure-storage-fuse/X86/disk/write/)      |
| Standard D96pds_v6 | [Throughput](https://azure.github.io/azure-storage-fuse/ARM64/disk/read/)   | [Throughput](https://azure.github.io/azure-storage-fuse/ARM64/disk/write/)    |

### Storage
A `Premium Blob Storage Account` and a `Standard Blob Storage Account` in `eastus2` region were used to conduct all tests. HNS was disabled on both these accounts.

### Blobfuse2 Configuration
Blobfuse2 is configured with file-cache or block-cache for all tests. These are two different caching strategies, each with distinct characteristics:

#### File Cache Configuration
[Configuration file: azure_key_perf.yaml](https://github.com/Azure/azure-storage-fuse/blob/main/testdata/config/azure_key_perf.yaml)

**How it works:**
- Downloads entire files from Azure Storage to local disk before allowing access
- Subsequent reads are served from the local cached copy
- Uses a local temporary directory (e.g., `/mnt/localssd/tempcache`) to store complete files
- Files remain in cache based on timeout settings (default: 30 seconds in benchmark config)

**Key Settings:**
- `path`: Local directory for caching files (uses fast local SSD storage)
- `timeout-sec: 30`: Files older than 30 seconds are eligible for eviction
- `sync-to-flush: true`: Ensures data is synced to Azure Storage on flush operations
- `cleanup-on-start: true`: Clears cache directory on mount

**Best for:**
- Sequential read/write workloads
- Applications that read/write entire files
- Scenarios where local disk space is available
- Workloads with good cache hit ratios

**Configuration:** [azure_key_perf.yaml](https://github.com/Azure/azure-storage-fuse/blob/main/testdata/config/azure_key_perf.yaml)

#### Block Cache Configuration
[Configuration file: azure_block_bench.yaml](https://github.com/Azure/azure-storage-fuse/blob/main/testdata/config/azure_block_bench.yaml)

**How it works:**
- Downloads only the blocks (chunks) of data that are requested
- Caches blocks in memory rather than on disk
- Does not require a local temp directory
- Useful for random access patterns where only portions of files are accessed

**Key Settings:**
- `block-size-mb: 16`: Each cached block is 16MB in size
- Memory-based caching (no local disk required)
- Parallel block downloads for improved performance

**Best for:**
- Random read workloads
- Large files where only small portions are accessed
- Scenarios with limited local disk space
- Applications that seek within files

**Configuration:** [azure_block_bench.yaml](https://github.com/Azure/azure-storage-fuse/blob/main/testdata/config/azure_block_bench.yaml)

## Tests and Results
Master test script that simulates this benchmarking test suite is located [here](https://github.com/Azure/azure-storage-fuse/tree/main/perf_testing/scripts/fio_bench.sh). To execute a specific test case download the script and execute below command:
```
    fio_bench.sh <mount-path> <test-name> <cache-mode>
```
- Script expects Blobfuse2 is already install on the system.
- Script expects the config file is present in current directory with name `config.yaml`
- Install `fio` and `jq` before you execute the script
- Allowed `test-name` are: read / write
- Allowed `cache-mode` are: file_cache / block_cache

### How Benchmark Tests Work
The benchmark suite performs the following steps for each test:
1. **Mount Blobfuse2**: Mounts the filesystem with the specified cache configuration
2. **Run FIO Tests**: Executes multiple FIO job files for read or write workloads
3. **Collect Metrics**: Captures bandwidth (throughput) and latency from FIO output
4. **Network Monitoring**: Tracks network usage (RX/TX bandwidth) during tests
5. **Aggregate Results**: Averages results across multiple iterations (typically 3 runs of 30 seconds each)
6. **Generate Reports**: Creates JSON files with bandwidth and latency summaries
7. **Publish Results**: Pushes results to gh-pages branch for visualization

### Understanding the Test Results

#### Bandwidth (Throughput)
- **Unit**: MiB/s (Mebibytes per second)
- **Higher is better**: Indicates how much data can be transferred per second
- **Interpretation**: 
  - Higher bandwidth means faster data transfer
  - File-cache typically shows higher sequential read bandwidth due to local caching
  - Block-cache may show better bandwidth for random reads on large files
  - Network bandwidth and storage account limits can cap throughput

#### Latency
- **Unit**: milliseconds (ms)
- **Lower is better**: Indicates how long operations take to complete
- **Interpretation**:
  - Lower latency means faster response times
  - File-cache usually has lower latency for cached files (near-disk speeds)
  - Block-cache latency depends on whether blocks are cached in memory
  - First-time reads have higher latency (network + storage latency)
  - Cached reads have much lower latency

#### Reading the Charts
The published charts on gh-pages show:
- **X-axis**: Commit hash or date of the test run
- **Y-axis**: Bandwidth (MiB/s) or Latency (ms)
- **Trend Lines**: Help visualize performance changes over time
- **Multiple Test Cases**: Each line represents a different FIO test configuration

#### What to Look For
- **Performance Regressions**: Sudden drops in bandwidth or increases in latency
- **Improvements**: Increases in bandwidth or decreases in latency after optimizations
- **Stability**: Consistent results indicate stable performance
- **Cache Impact**: Compare file_cache vs block_cache results for your workload type

Below table provides `latency/time` and `bandwidth` results for various tests on respective account types. Each test has a linked section describing details of that test case.

### VM: Standard D96ds_v5 (96 vcpus, 384 GiB memory) (X86_64) Results

#### Using file-cache:

| Storage Account Type | Read Performance                                                                                 | Write Performance                                                                                  |
| :------------------- | :----------------------------------------------------------------------------------------------- | :------------------------------------------------------------------------------------------------- |
| **Standard**         | [Throughput](https://azure.github.io/azure-storage-fuse/X86/standard/file_cache/bandwidth/read/) | [Throughput](https://azure.github.io/azure-storage-fuse/X86/standard/file_cache/bandwidth/write/)  |
| **Premium**          | [Throughput](https://azure.github.io/azure-storage-fuse/X86/premium/file_cache/bandwidth/read/)  | [Throughput](https://azure.github.io/azure-storage-fuse/X86/premium/file_cache/bandwidth/write/)   |

#### Using block-cache:

| Storage Account Type | Read Performance                                                                                  | Write Performance                                                                                   |
| :------------------- | :------------------------------------------------------------------------------------------------ | :-------------------------------------------------------------------------------------------------- |
| **Standard**         | [Throughput](https://azure.github.io/azure-storage-fuse/X86/standard/block_cache/bandwidth/read/) | [Throughput](https://azure.github.io/azure-storage-fuse/X86/standard/block_cache/bandwidth/write/)  |
| **Premium**          | [Throughput](https://azure.github.io/azure-storage-fuse/X86/premium/block_cache/bandwidth/read/)  | [Throughput](https://azure.github.io/azure-storage-fuse/X86/premium/block_cache/bandwidth/write/)   |

<!-- | Test Name | Premium Blob Storage Account | Standard Blob Storage Account |  -->
<!-- | ----------- | -------------- | ----------- | -->
<!-- | [Create](https://azure.github.io/azure-storage-fuse/#create)    |  [Latency](https://azure.github.io/azure-storage-fuse/X86/premium/latency/create/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/X86/premium/bandwidth/create/)  |  [Latency](https://azure.github.io/azure-storage-fuse/X86/standard/latency/create/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/X86/standard/bandwidth/create/) | -->
<!-- | [Write](https://azure.github.io/azure-storage-fuse/#write)     |  [Latency](https://azure.github.io/azure-storage-fuse/X86/premium/latency/write/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/X86/premium/bandwidth/write/)   | [Latency](https://azure.github.io/azure-storage-fuse/X86/standard/latency/write/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/X86/standard/bandwidth/write/) |  -->
<!-- | [Read](https://azure.github.io/azure-storage-fuse/#read)     |  [Latency](https://azure.github.io/azure-storage-fuse/X86/premium/latency/read/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/X86/premium/bandwidth/read/)    |  [Latency](https://azure.github.io/azure-storage-fuse/X86/standard/latency/read/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/X86/standard/bandwidth/read/) |  -->
<!-- | [List](https://azure.github.io/azure-storage-fuse/#list)      |  [Time](https://azure.github.io/azure-storage-fuse/X86/premium/time/list/)         |  [Time](https://azure.github.io/azure-storage-fuse/X86/standard/time/list/)    |  -->
<!-- | [Rename](https://azure.github.io/azure-storage-fuse/#rename)    |  [Time](https://azure.github.io/azure-storage-fuse/X86/premium/time/rename/)       |  [Time](https://azure.github.io/azure-storage-fuse/X86/standard/time/rename/)   |  -->
<!-- | [Highly Parallel](https://azure.github.io/azure-storage-fuse/#highly-parallel) |  [Latency](https://azure.github.io/azure-storage-fuse/X86/premium/latency/highlyparallel/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/X86/premium/bandwidth/highlyparallel/)  | [Latency](https://azure.github.io/azure-storage-fuse/X86/standard/latency/highlyparallel/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/X86/standard/bandwidth/highlyparallel/) |  -->
<!-- | [Application](https://azure.github.io/azure-storage-fuse/#application-test)       |  [Time](https://azure.github.io/azure-storage-fuse/X86/premium/time/app/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/X86/premium/bandwidth/app/) |  [Time](https://azure.github.io/azure-storage-fuse/X86/standard/time/app/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/X86/standard/bandwidth/app/) | -->
<!-- | [Max Out](https://azure.github.io/azure-storage-fuse/#max-out)       |  [Time](https://azure.github.io/azure-storage-fuse/X86/premium/time/highapp/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/X86/premium/bandwidth/highapp/) |  [Time](https://azure.github.io/azure-storage-fuse/X86/standard/time/highapp/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/X86/standard/bandwidth/highapp/) |   -->


### VM: Standard D96pds_v6 (96 vcpus, 384 GiB memory) (ARM64) Results

#### Using file-cache:

| Storage Account Type | Read Performance                                                                                   | Write Performance                                                                                    |
| :------------------- | :------------------------------------------------------------------------------------------------- | :--------------------------------------------------------------------------------------------------- |
| **Standard**         | [Throughput](https://azure.github.io/azure-storage-fuse/ARM64/standard/file_cache/bandwidth/read/) | [Throughput](https://azure.github.io/azure-storage-fuse/ARM64/standard/file_cache/bandwidth/write/)  |
| **Premium**          | [Throughput](https://azure.github.io/azure-storage-fuse/ARM64/premium/file_cache/bandwidth/read/)  | [Throughput](https://azure.github.io/azure-storage-fuse/ARM64/premium/file_cache/bandwidth/write/)   |

#### Using block-cache:

| Storage Account Type | Read Performance                                                                                    | Write Performance                                                                                     |
| :------------------- | :-------------------------------------------------------------------------------------------------- | :---------------------------------------------------------------------------------------------------- |
| **Standard**         | [Throughput](https://azure.github.io/azure-storage-fuse/ARM64/standard/block_cache/bandwidth/read/) | [Throughput](https://azure.github.io/azure-storage-fuse/ARM64/standard/block_cache/bandwidth/write/)  |
| **Premium**          | [Throughput](https://azure.github.io/azure-storage-fuse/ARM64/premium/block_cache/bandwidth/read/)  | [Throughput](https://azure.github.io/azure-storage-fuse/ARM64/premium/block_cache/bandwidth/write/)   |


<!-- | Test Name | Premium Blob Storage Account | Standard Blob Storage Account |  -->
<!-- | ----------- | -------------- | ----------- | -->
<!-- | [Create](https://azure.github.io/azure-storage-fuse/#create)    |  [Latency](https://azure.github.io/azure-storage-fuse/ARM/premium/latency/create/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/ARM/premium/bandwidth/create/)  |  [Latency](https://azure.github.io/azure-storage-fuse/ARM/standard/latency/create/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/ARM/standard/bandwidth/create/) | -->
<!-- | [Write](https://azure.github.io/azure-storage-fuse/#write)     |  [Latency](https://azure.github.io/azure-storage-fuse/ARM/premium/latency/write/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/ARM/premium/bandwidth/write/)   | [Latency](https://azure.github.io/azure-storage-fuse/ARM/standard/latency/write/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/ARM/standard/bandwidth/write/) |  -->
<!-- | [Read](https://azure.github.io/azure-storage-fuse/#read)     |  [Latency](https://azure.github.io/azure-storage-fuse/ARM/premium/latency/read/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/ARM/premium/bandwidth/read/)    |  [Latency](https://azure.github.io/azure-storage-fuse/ARM/standard/latency/read/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/ARM/standard/bandwidth/read/) |  -->
<!-- | [List](https://azure.github.io/azure-storage-fuse/#list)      |  [Time](https://azure.github.io/azure-storage-fuse/ARM/premium/time/list/)         |  [Time](https://azure.github.io/azure-storage-fuse/ARM/standard/time/list/)    |  -->
<!-- | [Rename](https://azure.github.io/azure-storage-fuse/#rename)    |  [Time](https://azure.github.io/azure-storage-fuse/ARM/premium/time/rename/)       |  [Time](https://azure.github.io/azure-storage-fuse/ARM/standard/time/rename/)   |  -->
<!-- | [Highly Parallel](https://azure.github.io/azure-storage-fuse/#highly-parallel) |  [Latency](https://azure.github.io/azure-storage-fuse/ARM/premium/latency/highlyparallel/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/ARM/premium/bandwidth/highlyparallel/)  | [Latency](https://azure.github.io/azure-storage-fuse/ARM/standard/latency/highlyparallel/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/ARM/standard/bandwidth/highlyparallel/) |  -->
<!-- | [Application](https://azure.github.io/azure-storage-fuse/#application-test)       |  [Time](https://azure.github.io/azure-storage-fuse/ARM/premium/time/app/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/ARM/premium/bandwidth/app/) |  [Time](https://azure.github.io/azure-storage-fuse/ARM/standard/time/app/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/ARM/standard/bandwidth/app/) | -->
<!-- | [Max Out](https://azure.github.io/azure-storage-fuse/#max-out)       |  [Time](https://azure.github.io/azure-storage-fuse/ARM/premium/time/highapp/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/ARM/premium/bandwidth/highapp/) |  [Time](https://azure.github.io/azure-storage-fuse/ARM/standard/time/highapp/) : [Bandwidth](https://azure.github.io/azure-storage-fuse/ARM/standard/bandwidth/highapp/) |   -->


<!-- ### Create -->
<!-- In this test `fio` command is used to create large number of small files in parallel. As part of the test `bandwidth` and `latency` are measured. Following cases are performed as part of this test: -->
<!-- ``` -->
<!--     - Create 1000 files of 1M each in 10 parallel threads -->
<!--     - Create 1000 files of 1M each in 100 parallel threads -->
<!--     - Create 100,000 files of 1K each in 20 parallel threads -->
<!-- ``` -->
<!-- All fio config files used during these tests are located [here](https://github.com/Azure/azure-storage-fuse/tree/main/perf_testing/config/create). -->

### Write Tests
In this test `fio` command is used to run various write workflows. As part of the test `bandwidth` and `latency` are measured. Each test is run for 30 seconds and average of 3 such iteration is taken. Both `bandwidth` and `latency` are taken from `fio` output directly and projected in the charts. 

#### Test Cases Executed:
The write benchmark runs the following FIO test configurations:

1. **Sequential Write** (`1_seq_write.fio`)
   - Writes a 100GB file sequentially
   - Block size: 1MB
   - Direct I/O enabled (`direct=1`)
   - Includes fsync after writes to capture flush time in throughput calculation
   - Tests sustained write performance with proper data durability

2. **Sequential Write - 16 Parallel Files** (`2_seq_write_16files.fio`)
   - Writes 16 different 100GB files in parallel using 16 threads
   - Tests multi-threaded write performance
   - Simulates workloads with concurrent file writes
   - Tests storage account's ability to handle parallel operations

All fio config files used during these tests are located [here](https://github.com/Azure/azure-storage-fuse/tree/main/perf_testing/config/write).

**Key Performance Factors:**
- File-cache: Writes to local disk first, then syncs to Azure Storage (can show higher initial throughput)
- Block-cache: Writes directly to Azure Storage in 16MB blocks
- Storage account type (Standard vs Premium) significantly impacts write performance
- Network bandwidth and storage account limits can cap write throughput

### Read Tests
In this test `fio` command is used to run various read workflows. As part of the test `bandwidth` and `latency` are measured. Each test is run for 30 seconds and average of 3 such iteration is taken. Both `bandwidth` and `latency` are taken from `fio` output directly and projected in the charts.

#### Test Cases Executed:
The read benchmark runs the following FIO test configurations:

1. **Sequential Read - Small File** (`1_seq_read_small.fio`)
   - Reads a 5MB file sequentially
   - Block size: 1MB
   - Tests small file read performance
   - Important for applications with many small files

2. **Random Read - Small File** (`2_rand_read_small.fio`)
   - Random reads from a 5MB file for 30 seconds
   - Block size: 1MB
   - Tests random access on small files

3. **Sequential Read** (`3_seq_read.fio`)
   - Reads a 100GB file sequentially
   - Block size: 1MB, Direct I/O enabled
   - Tests sustained sequential read throughput
   - Measures performance for large file streaming

4. **Random Read** (`4_rand_read.fio`)
   - Random reads from a 100GB file for 30 seconds
   - Block size: 1MB, Direct I/O enabled
   - Tests random access patterns on large files
   - Important for database-like workloads

5. **Sequential Read - 4 Threads** (`5_seq_read_4thread.fio`)
   - 4 parallel threads reading a 100GB file sequentially
   - Tests parallel read scalability
   - Each thread reads from the same file

6. **Sequential Read - 16 Threads** (`6_seq_read_16thread.fio`)
   - 16 parallel threads reading a 100GB file sequentially
   - Tests higher parallelism for read operations
   - Evaluates ability to saturate available bandwidth

7. **Random Read - 4 Threads** (`7_rand_read_4thread.fio`)
   - 4 parallel threads performing random reads on a 100GB file for 30 seconds
   - Tests concurrent random access patterns
   - Simulates multi-user random read scenarios

All fio config files used during these tests are located [here](https://github.com/Azure/azure-storage-fuse/tree/main/perf_testing/config/read).

**Key Performance Factors:**
- **File-cache**: 
  - First read downloads entire file to local disk (slower, network-limited)
  - Subsequent reads are very fast (local disk speeds)
  - Best for repeated sequential reads of the same file
- **Block-cache**: 
  - Downloads only requested blocks (16MB chunks)
  - Better for random reads where only parts of files are accessed
  - Lower cache storage requirements
  - More consistent performance across different access patterns
- **Cold vs Warm Cache**: First-time reads (cold cache) are always slower than cached reads
- **Storage Account Type**: Premium accounts provide lower latency and higher IOPS

<!-- ### List -->
<!-- In this test standard linux `ls -U --color=never` command is used to list all files on the mount path. Graphs are plotted based on total time taken by this operation. Post listing this test case also execute a delete operation to delete all files on the container and measures total time take to delete these files. -->
<!-- For the benchmarking purpose this test is executed after `Create` tests so that storage container has 100K+ files. -->

<!-- ### Rename -->
<!-- In this test 5000 files of 1MB size each are created using a python script and then each file is renamed using os.rename() method. Total time to rename all 5000 files is measured.  -->

<!-- ### Highly Parallel -->
<!-- In this test `fio` command is used to run various read/write workflows with high number of parallel threads. As part of the test `bandwidth` and `latency` are measured. Each test is run for 60 seconds and average of 3 such iteration is taken. Both `bandwidth` and `latency` are taken from `fio` output directly and projected in the charts. To simulate different read/write work-flows following cases are performed: -->
<!-- ``` -->
<!--     - Sequential write of 1G file each by 112 thread -->
<!--     - Sequential read of 1G file each by 128 thread -->
<!--     - Sequential read of 1G file each by 128 thread with direct-io -->
<!-- ``` -->
<!-- All fio config files used during these tests are located [here](https://github.com/Azure/azure-storage-fuse/tree/main/perf_testing/config/high_threads). -->

<!-- ### Application Test -->
<!-- We have observed that `fio` and `dd` commands have certain overhead of their own and they are not able to utilize our fuse solution to its full potential. With such tools you will observe tool itself consuming 100% CPU and performance of Blobfuse2 being blocked by the tool itself. To overcome this we created custom python applications to simulate sequential red/write of a large file.  -->

<!-- These applications are located [here](https://github.com/Azure/azure-storage-fuse/tree/main/perf_testing/scripts).  -->
<!-- Read/Write application is executed once and based on the time taken to read/write given amount of data bandwidth is computed.  -->

<!-- ### Max Out -->
<!-- Objective of this test case is to observe the max throughput that Blobfuse2 can generate. To achieve this two tests are run: -->
<!-- ``` -->
<!--     - Create 10 20GB files -->
<!--     - Read 10 20GB files -->
<!-- ``` -->

<!-- In case test 10 threads are run in parallel. Each thread is bound to a CPU core so that it gets max CPU for operation. 'dd' command is used to execute the test. -->

<!-- For write operation, data is read from '/dev/zero' in 16MB chunks and written to file on mounted path with 'direct io'.  -->

<!-- For read operations, data is read from mount path in 4MB chunks and sent to '/dev/null' with 'dd' set to not report any status.  -->

<!-- Test Scripts are located [here](https://github.com/Azure/azure-storage-fuse/tree/main/perf_testing/scripts/) with name "highspeed_". -->
