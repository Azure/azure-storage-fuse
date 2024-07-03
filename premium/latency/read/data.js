window.BENCHMARK_DATA = {
  "lastUpdate": 1719980380358,
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
          "id": "65a677429517c28a75a8ab6a3051311f44ba96aa",
          "message": "Revert back to parallel runs",
          "timestamp": "2024-07-02T20:58:56-07:00",
          "tree_id": "6f63f586f70486b7a8ef5ccd4a64d7862cb7715c",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/65a677429517c28a75a8ab6a3051311f44ba96aa"
        },
        "date": 1719980380139,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09738044743366668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 73.31157695015366,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.089245204077,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.17927439679066667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10913492414866666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 72.824497206219,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17504942334266668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9474936485596667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 75.40241367777966,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}