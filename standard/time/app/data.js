window.BENCHMARK_DATA = {
  "lastUpdate": 1719989711058,
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
        "date": 1719989710741,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.5847933292388916,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 5.523963212966919,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 52.05322504043579,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 19.79101300239563,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 1.0014736652374268,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 4.240872621536255,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 45.70890426635742,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 21.01099181175232,
            "unit": "seconds"
          }
        ]
      }
    ]
  }
}