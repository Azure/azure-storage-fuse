window.BENCHMARK_DATA = {
  "lastUpdate": 1768121843778,
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
        "date": 1768121843550,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 0.3473172227356667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 170.238744149627,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.2917636706596667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.26846526725966663,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.3429245718096667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 170.157207245676,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.43984044427033336,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.6229122706863333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 167.322518361546,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}