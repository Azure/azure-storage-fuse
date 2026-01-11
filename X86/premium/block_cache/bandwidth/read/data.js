window.BENCHMARK_DATA = {
  "lastUpdate": 1768106095148,
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
        "date": 1768106093619,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 2938.5569661458335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 14.708658854166666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 4100.684895833333,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 4658.921875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 3295.9342447916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 14.579752604166666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 8191.379231770833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 9412.102864583334,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 55.044596354166664,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}