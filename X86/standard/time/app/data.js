window.BENCHMARK_DATA = {
  "lastUpdate": 1741430312905,
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
        "date": 1741430312663,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.5611395835876465,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 6.151558876037598,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 56.29933023452759,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 20.487149000167847,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.7871878147125244,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 4.810240983963013,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 60.44978451728821,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 17.3947274684906,
            "unit": "seconds"
          }
        ]
      }
    ]
  }
}