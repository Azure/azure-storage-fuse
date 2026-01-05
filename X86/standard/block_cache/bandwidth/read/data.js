window.BENCHMARK_DATA = {
  "lastUpdate": 1767620303755,
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
        "date": 1767620302227,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2635.4508463541665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.1481119791666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2907.9860026041665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1421.4716796875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2553.6780598958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.1025390625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 5069.95703125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3426.8564453125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 4.489583333333333,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}