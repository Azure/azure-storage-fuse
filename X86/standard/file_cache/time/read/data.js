window.BENCHMARK_DATA = {
  "lastUpdate": 1768108806289,
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
        "date": 1768108806036,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 1.460735228498,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 1.928481692412,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.8679757108576668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.254561578106,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.9745988423006665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.2933306145776668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 1.0674669920546667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 2.520467980106,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 1.305248551118,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}