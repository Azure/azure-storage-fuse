window.BENCHMARK_DATA = {
  "lastUpdate": 1767620305157,
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
          "id": "e8128aeb8cb4f9d4a047c0817569a2864f3376ca",
          "message": "modify benchmarks",
          "timestamp": "2026-01-05T09:12:19Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e8128aeb8cb4f9d4a047c0817569a2864f3376ca"
        },
        "date": 1767620304909,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.084042432686,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 217.58400923720035,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.071416263175,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.16810908760933332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.097624440946,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 226.73365742326834,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.155704872715,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1534204067873335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 222.09488657215732,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}