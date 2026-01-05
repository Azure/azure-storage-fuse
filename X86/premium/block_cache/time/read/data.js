window.BENCHMARK_DATA = {
  "lastUpdate": 1767614571287,
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
        "date": 1767614571035,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09082970849866667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 74.70538331862066,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07777947839066666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.202657125375,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09288460086633332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 74.92131207486433,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16327616416,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.8897426590103334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 79.55099360553534,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}