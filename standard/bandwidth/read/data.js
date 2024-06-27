window.BENCHMARK_DATA = {
  "lastUpdate": 1719485214830,
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
          "id": "eb76cd1603914937829d87512f5ae46f03bb139d",
          "message": "Updated",
          "timestamp": "2024-06-27T03:26:05-07:00",
          "tree_id": "59cb8c935adfa5c3dc8d51a5df4ae9fed760a198",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/eb76cd1603914937829d87512f5ae46f03bb139d"
        },
        "date": 1719485213734,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2304.0504557291665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 2.0146484375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 3256.8994140625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1266.3268229166667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2399.822265625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 2.0921223958333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4870.069661458333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3635.9899088541665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 8.260416666666666,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}