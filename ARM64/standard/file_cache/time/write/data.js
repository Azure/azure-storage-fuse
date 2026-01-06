window.BENCHMARK_DATA = {
  "lastUpdate": 1767706170751,
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
        "date": 1767706170523,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_write_kernel_cache",
            "value": 0.468457803953,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.5918418566536667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 1.2244949675463332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 4.574610461325666,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}