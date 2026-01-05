window.BENCHMARK_DATA = {
  "lastUpdate": 1767617533983,
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
        "date": 1767617532566,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 636.8297526041666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 381.7776692708333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 660.49609375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1188.365234375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 607.5338541666666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 594.9899088541666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 2098.2639973958335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1104.4013671875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 2208.6774088541665,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}