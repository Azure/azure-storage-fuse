window.BENCHMARK_DATA = {
  "lastUpdate": 1719920234999,
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
        "date": 1719920234776,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.097841479843,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 69.92813341352134,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09039146236366667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18724494861733332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09791863916833334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 71.92304481089366,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18251522856966665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.06746595329,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 72.32968782158633,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}