window.BENCHMARK_DATA = {
  "lastUpdate": 1719989713230,
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
        "date": 1719989712972,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "read_10_20GB_file",
            "value": 71.51457333564758,
            "unit": "seconds"
          },
          {
            "name": "create_10_20GB_file",
            "value": 52.305187463760376,
            "unit": "seconds"
          }
        ]
      }
    ]
  }
}