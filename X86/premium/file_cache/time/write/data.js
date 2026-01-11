window.BENCHMARK_DATA = {
  "lastUpdate": 1768104766669,
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
          "id": "bff4bcf063db1d95d3f8a7ba10b498226ce1afec",
          "message": "modify benchmarks",
          "timestamp": "2026-01-09T07:33:48Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/bff4bcf063db1d95d3f8a7ba10b498226ce1afec"
        },
        "date": 1768104766425,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_write_kernel_cache",
            "value": 0.7774383401186666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.9386857167586666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 1.0753690658263333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 3.2344467465306663,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}