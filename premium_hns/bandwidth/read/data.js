window.BENCHMARK_DATA = {
  "lastUpdate": 1719556500056,
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
        "date": 1719556499157,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2144.1337890625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.5286458333333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2374.5120442708335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1385.2864583333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2489.7861328125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.7613932291666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4978.787109375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3634.5716145833335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 14.033528645833334,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}