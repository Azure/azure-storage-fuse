window.BENCHMARK_DATA = {
  "lastUpdate": 1767638874925,
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
        "date": 1767638874673,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 0.353300925936,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 220.201233254369,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.34203888802366667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.224703053947,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.3235720417366667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 213.55420115254137,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.45468167219600003,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.73625057909,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 220.40052244204966,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}