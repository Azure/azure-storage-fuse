window.BENCHMARK_DATA = {
  "lastUpdate": 1767705031666,
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
          "id": "c1a8d67c576acfbf5e92cc8abb649364554c7ecc",
          "message": "modify benchmarks",
          "timestamp": "2026-01-05T09:12:19Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/c1a8d67c576acfbf5e92cc8abb649364554c7ecc"
        },
        "date": 1767705031428,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 0.8419536834416667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 1.1947014074620002,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.5875208801693333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.31959539187533337,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.6950590648396666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 0.6087449106399999,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.6281528056460001,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.666303207707,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 0.5731050359183333,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}