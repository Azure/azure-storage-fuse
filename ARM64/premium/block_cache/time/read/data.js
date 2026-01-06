window.BENCHMARK_DATA = {
  "lastUpdate": 1767702595701,
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
          "id": "c1a8d67c576acfbf5e92cc8abb649364554c7ecc",
          "message": "modify benchmarks",
          "timestamp": "2026-01-05T09:12:19Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/c1a8d67c576acfbf5e92cc8abb649364554c7ecc"
        },
        "date": 1767702595465,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 0.30043093757633327,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 68.30103588640434,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.242845058914,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.2676714795203334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.304646213574,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 74.18598979008134,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.3933710265556667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.64758782951,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 76.628539867318,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}