window.BENCHMARK_DATA = {
  "lastUpdate": 1768116197388,
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
        "date": 1768116197156,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 0.29554113586399994,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 61.58743316837067,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.2613629060123333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.286807791728,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.3021462133803334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 67.82171666181334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.4013353195896667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.6359968427173335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 70.18996200386,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}