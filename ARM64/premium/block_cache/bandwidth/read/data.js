window.BENCHMARK_DATA = {
  "lastUpdate": 1767702594404,
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
          "id": "c1a8d67c576acfbf5e92cc8abb649364554c7ecc",
          "message": "modify benchmarks",
          "timestamp": "2026-01-05T09:12:19Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/c1a8d67c576acfbf5e92cc8abb649364554c7ecc"
        },
        "date": 1767702592906,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 2867.0595703125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 14.6943359375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 4116.084309895833,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 3728.2197265625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 3280.345703125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 13.488932291666666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 10130.6171875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 9596.668619791666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 52.194010416666664,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}