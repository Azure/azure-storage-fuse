window.BENCHMARK_DATA = {
  "lastUpdate": 1767632569518,
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
        "date": 1767632569202,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_write_kernel_cache",
            "value": 1388.0231119791667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 1206.4557291666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 4782.205729166667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 5746.639973958333,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}