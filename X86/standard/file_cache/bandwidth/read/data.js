window.BENCHMARK_DATA = {
  "lastUpdate": 1768108804908,
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
        "date": 1768108803440,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 346.2171223958333,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 150.1904296875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 1126.8916015625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 3928.2574869791665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 449.9807942708333,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 182.87858072916666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 1198.9235026041667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1041.0472005208333,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 779.0777994791666,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}