# Blobfuse2 Health Monitor (Preview)

## About

Blobfuse2 Health Monitor is a tool which will help in monitoring Blobfuse2 mounts. It supports the following types of monitors:

1. **Blobfuse2 Stats Monitor:** Monitor the different statistics of blobfuse2 components like,
    - Total bytes uploaded and downloaded via blobfuse2
    - Events like create, delete, rename, synchronize, truncate, etc. on files or directories in the mounted directory
    - Progress of uploads or downloads of large files to/from Azure Storage
    - Keep track of number of calls that were made to Azure Storage for operations like create, delete, rename, chmod, etc. in the mounted directory
    - Total number of open handles on files
    - Number of times an open file request was served from the file cache or downloaded from the Azure Storage  

2. **CPU and Memory Monitor:** Monitor the CPU and memory usage of the Blobfuse2 process associated with the mount

3. **File Cache Monitor:** Monitor the file cache directory specified while mounting. This monitor does the following,
    - Monitor the different events like create, delete, rename, chmod, etc. of files and directories in the cache
    - Keep track of the cache consumption with respect to the cache size specified during mounting

> **Note:** Health Monitor runs as a separate process where one health monitor process is associated with monitoring one blobfuse2 mounted directory.

## Enable Health Monitor

The different configuration options for the health monitor are,
- `enable-monitoring: true|false`: Boolean parameter to enable health monitor. By default it is disabled
- `stats-poll-interval-sec: <TIME IN SECONDS>`: Blobfuse2 stats polling interval (in sec). Default is 10 seconds
- `process-monitor-interval-sec: <TIME IN SECONDS>`: CPU and memory usage polling interval (in sec). Default is 30 sec
- `output-path: <PATH>`: Path where health monitor will generate its output file. It takes the current directory as default, if not specified. Output file name will be `monitor_<pid>.json`
- `monitor-disable-list: <LIST OF MONITORS>`: List of monitors to be disabled. To disable a monitor, add its corresponding name in the list
    - `blobfuse_stats` - Disable blobfuse2 stats polling
    - `cpu_profiler` - Disable CPU monitoring on blobfuse2 process
    - `memory_profiler` - Disable memory monitoring on blobfuse2 process
    - `file_cache_monitor` - Disable file cache directory monitor

### Sample Config

Add the following section to your blobfuse2 config file. Here file cache and memory monitors are disabled.
```yaml
health_monitor:
  enable-monitoring: true
  stats-poll-interval-sec: 10
  process-monitor-interval-sec: 30
  output-path: outputReportsPath
  monitor-disable-list:
    - file_cache_monitor
    - memory_profiler
```

## Output Reports

Health monitor will store its output reports in the path specified in the `output-path` config option. If this option is not specified, it takes the current directory as default. It stores the last 100MB of monitor data in 10 different files named as `monitor_<pid>_<index>.json` where `monitor_<pid>.json`(Zeroth index) is latest and `monitor_<pid>_9.json` is the oldest output file.

### Sample Output

```
{
    "Timestamp": "t1",
    "CPUUsage": "value in %",
    "MemoryUsage": "value in bytes",
    "BlobfuseStats": [
        {
            "componentName": "azstorage",
            "value": {
                "Bytes Downloaded": value in bytes,
                "Bytes Uploaded": value in bytes,
                "Chmod": count of chmod calls,
                "StreamDir": count of stream dir calls
            }
        },
        {
            "componentName": "file_cache",
            "value": {
                "Cache Usage": "value in MB",
                "Usage Percent": "value in %",
                "Files Downloaded": count,
                "Files served from cache": count
            }
        }
    ],
    "FileCache": [
        {
            "cacheEvent": "CREATE",
            "path": "filePath",
            "isDir": false,
            "cacheSize": value in bytes,
            "cacheConsumed": "value in %",
            "cacheFilesCount": count of files in cache,
            "evictedFilesCount": count of files evicted from cache,
            "value": {
                "FileSize": "value in bytes"
            }
        }
    ]
}
```



