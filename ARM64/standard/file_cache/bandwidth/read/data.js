window.BENCHMARK_DATA = {
  "lastUpdate": 1768118915777,
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
        "date": 1768118913954,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 708.2740885416666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 248.09244791666666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 1054.1070963541667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 3008.6217447916665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 662.1979166666666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 469.5598958333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 3820.8772786458335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 18391.192057291668,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 3038.55859375,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}