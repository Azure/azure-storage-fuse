window.BENCHMARK_DATA = {
  "lastUpdate": 1719485191868,
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
        "date": 1719485190848,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2295.5423177083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.6461588541666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2873.6207682291665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1173.3492838541667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2424.6350911458335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.5540364583333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4707.40234375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3502.2604166666665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.375325520833334,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}