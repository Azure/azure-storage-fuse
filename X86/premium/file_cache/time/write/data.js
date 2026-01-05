window.BENCHMARK_DATA = {
  "lastUpdate": 1767632570871,
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
        "date": 1767632570623,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_write_kernel_cache",
            "value": 0.719367709453,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.8322059806013334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.8361658734813333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2.7820254129106665,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}