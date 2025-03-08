window.BENCHMARK_DATA = {
  "lastUpdate": 1741432080499,
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
        "date": 1741432079185,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2715.4977213541665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.998046875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2521.4124348958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1375.5895182291667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2456.2457682291665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 4.345052083333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 6722.8935546875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 6985.203450520833,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 15.16796875,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}