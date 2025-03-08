window.BENCHMARK_DATA = {
  "lastUpdate": 1741438728192,
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
          "id": "03e72e47d37985e5c28051c0ff17bdc0c7315e74",
          "message": "Correcting code for cache cleanup",
          "timestamp": "2025-03-07T23:48:22-08:00",
          "tree_id": "5b69276c81c0c728ae2dd3889b9743194fdcc990",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/03e72e47d37985e5c28051c0ff17bdc0c7315e74"
        },
        "date": 1741438727898,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 0.9445252418518066,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 3.836538076400757,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 37.9499990940094,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 15.608469247817993,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.8138916492462158,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.811798095703125,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 37.514626026153564,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 14.37084698677063,
            "unit": "seconds"
          }
        ]
      }
    ]
  }
}