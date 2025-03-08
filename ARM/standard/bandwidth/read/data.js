window.BENCHMARK_DATA = {
  "lastUpdate": 1741440595023,
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
        "date": 1741440593850,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2724.0729166666665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.8990885416666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2578.6194661458335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1415.2275390625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2430.28515625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.986328125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 6499.58203125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 6758.802083333333,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.912434895833333,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}