window.BENCHMARK_DATA = {
  "lastUpdate": 1741432081620,
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
        "date": 1741432081386,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.08212261071666667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 62.559968798558,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08478790776066668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.166485268844,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10162477885633332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 57.636108702455665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.12647266981633334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.5496881590483333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 65.85110394025433,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}