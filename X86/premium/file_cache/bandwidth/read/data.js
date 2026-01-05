window.BENCHMARK_DATA = {
  "lastUpdate": 1767611764522,
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
          "id": "e8128aeb8cb4f9d4a047c0817569a2864f3376ca",
          "message": "modify benchmarks",
          "timestamp": "2026-01-05T09:12:19Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e8128aeb8cb4f9d4a047c0817569a2864f3376ca"
        },
        "date": 1767611763064,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 1458.0221354166667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 565.8212890625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 1804.8229166666667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1076.0553385416667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1380.1943359375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1104.8424479166667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 3528.5989583333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 2429.087890625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 2514.6396484375,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}