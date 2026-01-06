window.BENCHMARK_DATA = {
  "lastUpdate": 1767707756531,
  "repoUrl": "https://github.com/Azure/azure-storage-fuse",
  "entries": {
    "Benchmark": [
      {
        "commit": {
          "author": {
            "name": "Srinivas Yeleti",
            "username": "syeleti-msft",
            "email": "syeleti@microsoft.com"
          },
          "committer": {
            "name": "Srinivas Yeleti",
            "username": "syeleti-msft",
            "email": "syeleti@microsoft.com"
          },
          "id": "81eb4bd71f6cbb7f3116c9c12b670a9321c2cea4",
          "message": "modify benchmarks",
          "timestamp": "2026-01-05T09:12:19Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/81eb4bd71f6cbb7f3116c9c12b670a9321c2cea4"
        },
        "date": 1767707755050,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 2544.1927083333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 6.343098958333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 3529.0436197916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 3510.5045572916665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2846.8932291666665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 6.417317708333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 8957.284830729166,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 9596.4501953125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 24.527669270833332,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}