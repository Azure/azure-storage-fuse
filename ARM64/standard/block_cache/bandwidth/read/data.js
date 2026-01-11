window.BENCHMARK_DATA = {
  "lastUpdate": 1768121842492,
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
        "date": 1768121840757,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 2873.4593098958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 5.876953125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 3424.0188802083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 3722.1975911458335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2917.5358072916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 5.879231770833333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 9065.998046875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 9625.455078125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 23.841145833333332,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}