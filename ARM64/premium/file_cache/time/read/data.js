window.BENCHMARK_DATA = {
  "lastUpdate": 1768113159292,
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
        "date": 1768113159060,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 0.7809711043089999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 1.2317959557759999,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.6891741302176667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.32335147358800004,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.7482126282193334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 0.7253901663016666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.689376794651,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.655260941968,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 0.6639332567790001,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}