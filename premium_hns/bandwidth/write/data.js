window.BENCHMARK_DATA = {
  "lastUpdate": 1719558088612,
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
        "date": 1719558088296,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 1853.61328125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 1807.294921875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 1878.4007161458333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2017.6627604166667,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}