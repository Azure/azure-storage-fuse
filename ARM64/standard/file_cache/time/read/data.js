window.BENCHMARK_DATA = {
  "lastUpdate": 1768118917094,
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
        "date": 1768118916854,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 0.821992566514,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 1.261365999403,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.730476810657,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.33105328759633335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.7389156549829999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 0.7411434533653333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.6955713558573334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.6819029766110001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 0.6349262951863333,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}