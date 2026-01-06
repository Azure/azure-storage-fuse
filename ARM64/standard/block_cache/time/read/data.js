window.BENCHMARK_DATA = {
  "lastUpdate": 1767707757910,
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
          "id": "81eb4bd71f6cbb7f3116c9c12b670a9321c2cea4",
          "message": "modify benchmarks",
          "timestamp": "2026-01-05T09:12:19Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/81eb4bd71f6cbb7f3116c9c12b670a9321c2cea4"
        },
        "date": 1767707757674,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 0.34556333927433336,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 157.65625816542067,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.28306824798766667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.284708614918,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.3509945339493334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 155.81308813885366,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.44638784205666665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.637696014902,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 159.097908381141,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}