window.BENCHMARK_DATA = {
  "lastUpdate": 1719984017724,
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
        "date": 1719984017424,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 5341.404193015179,
            "unit": "MiB/s"
          },
          {
            "name": "write_10GB",
            "value": 14836.069476121218,
            "unit": "MiB/s"
          },
          {
            "name": "write_100GB",
            "value": 13454.032418799794,
            "unit": "MiB/s"
          },
          {
            "name": "write_40GB",
            "value": 15675.569692170726,
            "unit": "MiB/s"
          },
          {
            "name": "read_1GB",
            "value": 12260.824766826683,
            "unit": "MiB/s"
          },
          {
            "name": "read_10GB",
            "value": 19687.817121790154,
            "unit": "MiB/s"
          },
          {
            "name": "read_100GB",
            "value": 20360.355082167884,
            "unit": "MiB/s"
          },
          {
            "name": "read_40GB",
            "value": 15788.863875893048,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}