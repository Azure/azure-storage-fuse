window.BENCHMARK_DATA = {
  "lastUpdate": 1767633875685,
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
        "date": 1767633874246,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 2472.0172526041665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 13.353515625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 3901.05859375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 4404.097005208333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 3101.2962239583335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 12.681640625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 8957.5673828125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 8243.1923828125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 48.892903645833336,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}