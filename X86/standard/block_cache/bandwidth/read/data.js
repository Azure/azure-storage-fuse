window.BENCHMARK_DATA = {
  "lastUpdate": 1768111221046,
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
        "date": 1768111219539,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 2462.4397786458335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 5.6611328125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 4183.6044921875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 4508.882486979167,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2766.99609375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 5.611328125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 7007.2216796875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 8654.811848958334,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 22.595052083333332,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}