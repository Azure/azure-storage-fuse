window.BENCHMARK_DATA = {
  "lastUpdate": 1767631364000,
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
        "date": 1767631362513,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 763.9593098958334,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 404.0934244791667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 1229.1100260416667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 3711.7311197916665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 840.9899088541666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1064.7711588541667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 3609.7867838541665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 12431.632486979166,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 3379.2054036458335,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}