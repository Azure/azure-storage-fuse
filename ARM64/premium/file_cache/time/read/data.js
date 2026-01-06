window.BENCHMARK_DATA = {
  "lastUpdate": 1767678815124,
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
          "id": "43454e481b177e52c67637d6392942dc6dddba11",
          "message": "modify benchmarks",
          "timestamp": "2026-01-05T09:12:19Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/43454e481b177e52c67637d6392942dc6dddba11"
        },
        "date": 1767678814897,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 0.9530113437833334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 1.2680121148646668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.767426043835,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.36310981791700003,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.7392187168346666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 0.7159923335753334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.706610171327,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.7156014903816666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 0.631993851231,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}