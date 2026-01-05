window.BENCHMARK_DATA = {
  "lastUpdate": 1767638873588,
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
          "id": "af160039bb50ceedeadb8e35d831a6d352de1395",
          "message": "modify benchmarks",
          "timestamp": "2026-01-05T09:12:19Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/af160039bb50ceedeadb8e35d831a6d352de1395"
        },
        "date": 1767638872113,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 2519.7158203125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 4.539713541666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 3060.2975260416665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 4439.20703125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 3092.0673828125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 4.682942708333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 8778.91796875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 9182.187825520834,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 18.121744791666668,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}