window.BENCHMARK_DATA = {
  "lastUpdate": 1767620800870,
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
        "date": 1767620800603,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 0.128232144574,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.13648678963566666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.12704964809066666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.12847842279433333,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}