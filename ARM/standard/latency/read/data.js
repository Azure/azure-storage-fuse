window.BENCHMARK_DATA = {
  "lastUpdate": 1741440596904,
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
        "date": 1741440596676,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.08194738391366667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 131.58380015690534,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08287511748866666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.161714316194,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10273841746799998,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 125.926002856735,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.132631582201,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.5695571346166667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 126.16549849148068,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}