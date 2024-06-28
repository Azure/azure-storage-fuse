window.BENCHMARK_DATA = {
  "lastUpdate": 1719556608994,
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
        "date": 1719556608134,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2242.5719401041665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 2.53515625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2282.220703125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1337.8414713541667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2445.8180338541665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 2.505859375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4710.9599609375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3537.3665364583335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 9.7138671875,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}