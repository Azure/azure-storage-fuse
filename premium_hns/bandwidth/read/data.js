window.BENCHMARK_DATA = {
  "lastUpdate": 1719948929888,
  "repoUrl": "https://github.com/Azure/azure-storage-fuse",
  "entries": {
    "Benchmark": [
      {
        "commit": {
          "author": {
            "email": "vibhansa@microsoft.com",
            "name": "vibhansa",
            "username": "vibhansa-msft"
          },
          "committer": {
            "email": "vibhansa@microsoft.com",
            "name": "vibhansa",
            "username": "vibhansa-msft"
          },
          "distinct": true,
          "id": "559f3a7a508cbaf394bdcaea19e2868eeee8e81d",
          "message": "Updated",
          "timestamp": "2024-07-02T04:06:54-07:00",
          "tree_id": "5049cd24972625012757215aec88556fa6c492f1",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/559f3a7a508cbaf394bdcaea19e2868eeee8e81d"
        },
        "date": 1719948929617,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2258.8489583333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.7613932291666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2414.388671875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1198.5149739583333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2365.5205078125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.5836588541666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4417.541341145833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3440.9055989583335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 12.878580729166666,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}