window.BENCHMARK_DATA = {
  "lastUpdate": 1768114440483,
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
        "date": 1768114440244,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_write_kernel_cache",
            "value": 0.5118407725406666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.6514909980580001,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.6442987066336667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 1.9470781271900002,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}