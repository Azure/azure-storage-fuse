window.BENCHMARK_DATA = {
  "lastUpdate": 1768103617553,
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
      }
    ]
  }
}