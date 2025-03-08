window.BENCHMARK_DATA = {
  "lastUpdate": 1741424452474,
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
        "date": 1741424452161,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 0.9691689014434814,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 5.4485390186309814,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 48.71264410018921,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 20.362226247787476,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.7030744552612305,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 4.30452561378479,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 51.96659541130066,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 21.037559986114502,
            "unit": "seconds"
          }
        ]
      }
    ]
  }
}