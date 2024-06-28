window.BENCHMARK_DATA = {
  "lastUpdate": 1719556501178,
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
          "id": "dceabf98c8b22f702a6cf9bacaf35de84bef6df4",
          "message": "Export hns env variable",
          "timestamp": "2024-06-27T23:16:58-07:00",
          "tree_id": "6b5ecff628052a80e33170edda2655037d1a3033",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/dceabf98c8b22f702a6cf9bacaf35de84bef6df4"
        },
        "date": 1719556500949,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.105959399281,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 71.02136294327101,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09109557004333335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.16586756297933333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10027042514033334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 66.78191125887567,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16344805206433333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0711787437233333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 71.17170611462001,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}