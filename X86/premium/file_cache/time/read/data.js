window.BENCHMARK_DATA = {
  "lastUpdate": 1768103618879,
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
        "date": 1768103618626,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 1.5024643161166666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 2.127287810482,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.9638722725056666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.268984925224,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.8474050082546666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.4342285589193333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 1.1551931150123333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 2.3009252351523335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 1.2374739347663333,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}