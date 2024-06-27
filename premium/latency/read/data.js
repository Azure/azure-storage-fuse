window.BENCHMARK_DATA = {
  "lastUpdate": 1719485193070,
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
        "date": 1719485192846,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09979287622066667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 68.581276523296,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07682404527366667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.1987544916263333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.103779736424,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 70.3513079892,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18162104924233335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1089321731243333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 74.62924077265534,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}