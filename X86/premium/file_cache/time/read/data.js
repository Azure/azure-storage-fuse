window.BENCHMARK_DATA = {
  "lastUpdate": 1767611766079,
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
        "date": 1767611765827,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.3021191357963333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 0.49405806778233335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.12457904699833333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.22376350375600004,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.39588959157266673,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 0.388481427739,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.31144318398433335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.8963509745330001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 0.3853104458513334,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}