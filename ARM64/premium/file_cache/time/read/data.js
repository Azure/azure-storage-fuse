window.BENCHMARK_DATA = {
  "lastUpdate": 1768422933693,
  "repoUrl": "https://github.com/Azure/azure-storage-fuse",
  "entries": {
    "Benchmark": [
      {
        "commit": {
          "author": {
            "name": "syeleti-msft",
            "username": "syeleti-msft",
            "email": "syeleti@microsoft.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "cd2cb4303028ac433d46f7284e70869eb529a19e",
          "message": "Add parallel file writes to different files fio configs (#2096)",
          "timestamp": "2026-01-14T13:24:28Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/cd2cb4303028ac433d46f7284e70869eb529a19e"
        },
        "date": 1768422933455,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_small_file",
            "value": 0.31350075511700004,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.34632584962,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read",
            "value": 0.29258488084,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 0.869672,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.32661876895499997,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.29570279833999996,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 0.82396925,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}