window.BENCHMARK_DATA = {
  "lastUpdate": 1767637538360,
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
        "date": 1767637538103,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_write_kernel_cache",
            "value": 0.716359531035,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.8188753999253334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.8905071528859999,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2.82640877144,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}