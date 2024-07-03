window.BENCHMARK_DATA = {
  "lastUpdate": 1719984018878,
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
        "date": 1719984018597,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.5456609725952148,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 5.525991916656494,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 60.89356517791748,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 20.90794825553894,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.6733641624450684,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 4.164199590682983,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 40.238198041915894,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 20.75792169570923,
            "unit": "seconds"
          }
        ]
      }
    ]
  }
}