window.BENCHMARK_DATA = {
  "lastUpdate": 1741422669139,
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
        "date": 1741422668897,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 0.13235690869766667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.137772399535,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.13085574344466666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.12508921090733333,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}