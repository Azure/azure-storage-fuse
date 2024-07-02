window.BENCHMARK_DATA = {
  "lastUpdate": 1719920233856,
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
        "date": 1719920232928,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2335.9801432291665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.5774739583333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2484.4391276041665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1237.6276041666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2558.853515625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.4768880208333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4644.56640625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3659.1526692708335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.797526041666666,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}