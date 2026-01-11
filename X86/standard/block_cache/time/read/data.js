window.BENCHMARK_DATA = {
  "lastUpdate": 1768111222238,
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
        "date": 1768111221985,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 0.409433452879,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 176.646092986551,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.23954582622099999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.22128690754933333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.36413976058700004,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 178.17866939232866,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.569743506643,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.835239743124,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 176.35297410339567,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}