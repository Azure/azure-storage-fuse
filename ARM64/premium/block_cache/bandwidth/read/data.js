window.BENCHMARK_DATA = {
  "lastUpdate": 1768116196060,
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
        "date": 1768116194303,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 3157.4879557291665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 16.300455729166668,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 3822.4833984375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 3479.572265625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 3308.6634114583335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 14.744140625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 9940.377278645834,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 9691.237955729166,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 56.956705729166664,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}