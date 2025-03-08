window.BENCHMARK_DATA = {
  "lastUpdate": 1741422667978,
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
        "date": 1741422667689,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 1891.5738932291667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 1818.85546875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 1931.9270833333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 1994.5107421875,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}