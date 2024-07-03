window.BENCHMARK_DATA = {
  "lastUpdate": 1719989709800,
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
        "date": 1719989709559,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 5209.51208443375,
            "unit": "MiB/s"
          },
          {
            "name": "write_10GB",
            "value": 14841.51809837387,
            "unit": "MiB/s"
          },
          {
            "name": "write_100GB",
            "value": 15738.96716992237,
            "unit": "MiB/s"
          },
          {
            "name": "write_40GB",
            "value": 16560.243781373283,
            "unit": "MiB/s"
          },
          {
            "name": "read_1GB",
            "value": 8243.851322882952,
            "unit": "MiB/s"
          },
          {
            "name": "read_10GB",
            "value": 19331.87042300302,
            "unit": "MiB/s"
          },
          {
            "name": "read_100GB",
            "value": 17923.509940775217,
            "unit": "MiB/s"
          },
          {
            "name": "read_40GB",
            "value": 15598.692481364882,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}