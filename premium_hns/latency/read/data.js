window.BENCHMARK_DATA = {
  "lastUpdate": 1719948930967,
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
        "date": 1719948930736,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.100591923383,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 66.485799230399,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08920023977833332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.19545377686433332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10589950805433333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 69.79609327705967,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.19061023523466666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1396544670470001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 77.51946139900033,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}