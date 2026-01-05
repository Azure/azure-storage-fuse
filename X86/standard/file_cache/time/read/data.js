window.BENCHMARK_DATA = {
  "lastUpdate": 1767617535297,
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
        "date": 1767617535029,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.7827057626426667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 0.5512607561406666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.9546172787856667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.201939958524,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.5013156247956667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 0.44544505053766664,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.44930101993399996,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.8858039689093333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 0.47534748656000003,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}