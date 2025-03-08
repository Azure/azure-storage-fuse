window.BENCHMARK_DATA = {
  "lastUpdate": 1741426021649,
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
        "date": 1741426020521,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2335.1077473958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.6907552083333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 3227.5419921875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1252.4521484375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2206.4817708333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.6272786458333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4597.584635416667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3836.5074869791665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 6.67578125,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}