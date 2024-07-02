window.BENCHMARK_DATA = {
  "lastUpdate": 1719943825782,
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
        "date": 1719943825527,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2397.7610677083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 2.0231119791666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2947.431640625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1605.8346354166667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2504.8912760416665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 2.1832682291666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4829.162434895833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3478.1650390625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 8.5361328125,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}