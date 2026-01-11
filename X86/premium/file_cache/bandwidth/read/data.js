window.BENCHMARK_DATA = {
  "lastUpdate": 1768149922090,
  "repoUrl": "https://github.com/Azure/azure-storage-fuse",
  "entries": {
    "Benchmark": [
      {
        "commit": {
          "author": {
            "name": "Srinivas Yeleti",
            "username": "syeleti-msft",
            "email": "syeleti@microsoft.com"
          },
          "committer": {
            "name": "Srinivas Yeleti",
            "username": "syeleti-msft",
            "email": "syeleti@microsoft.com"
          },
          "id": "bff4bcf063db1d95d3f8a7ba10b498226ce1afec",
          "message": "modify benchmarks",
          "timestamp": "2026-01-09T07:33:48Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/bff4bcf063db1d95d3f8a7ba10b498226ce1afec"
        },
        "date": 1768103617093,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 351.5071614583333,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 141.90234375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 1388.5830078125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 3700.0983072916665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 427.5934244791667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 230.85481770833334,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 1076.8834635416667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1174.2477213541667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 857.0638020833334,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "Srinivas Yeleti",
            "username": "syeleti-msft",
            "email": "syeleti@microsoft.com"
          },
          "committer": {
            "name": "Srinivas Yeleti",
            "username": "syeleti-msft",
            "email": "syeleti@microsoft.com"
          },
          "id": "e3a08c0649fd263abfb4746f0f7629695f8450d0",
          "message": "modify benchmarks",
          "timestamp": "2026-01-09T10:09:46Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e3a08c0649fd263abfb4746f0f7629695f8450d0"
        },
        "date": 1768129398766,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 369.7509765625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 143.0087890625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 840.423828125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 3470.1617838541665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 361.5953776041667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 194.46126302083334,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 1226.7454427083333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1053.0302734375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 848.6848958333334,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "syeleti-msft",
            "username": "syeleti-msft",
            "email": "syeleti@microsoft.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "446f5da15149304940ed01d95637d2e3d035fe16",
          "message": "Remove getting size from statfs (#2083)\n\nCo-authored-by: Vikas Bhansali <64532198+vibhansa-msft@users.noreply.github.com>",
          "timestamp": "2026-01-09T09:42:45Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/446f5da15149304940ed01d95637d2e3d035fe16"
        },
        "date": 1768149921841,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 746.3821614583334,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 383.1494140625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 1055.6829427083333,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 4019.5875651041665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1008.2180989583334,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 947.7639973958334,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 3700.5615234375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 13712.646158854166,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 3549.9674479166665,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}