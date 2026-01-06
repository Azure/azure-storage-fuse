window.BENCHMARK_DATA = {
  "lastUpdate": 1767705030241,
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
        "date": 1767705028811,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 1080.0615234375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 505.1695963541667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 1221.134765625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 3121.8177083333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1070.017578125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1076.1842447916667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4580.405924479167,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 18894.390950520832,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 4353.4033203125,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}