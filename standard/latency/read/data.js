window.BENCHMARK_DATA = {
  "lastUpdate": 1719943826972,
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
        "date": 1719943826740,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.093688619147,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 123.569887442136,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07124905374066667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.14071839190933333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10026112907533334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 114.47500333939666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17408663255800003,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1284134373766666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 116.81310448915333,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}