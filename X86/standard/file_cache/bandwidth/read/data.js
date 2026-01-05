window.BENCHMARK_DATA = {
  "lastUpdate": 1767636351070,
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
        "date": 1767636349637,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 764.2037760416666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 317.8740234375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 1282.9866536458333,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 3503.5826822916665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 723.4827473958334,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 675.05078125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 3732.0615234375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 11817.610026041666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 3293.4085286458335,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}