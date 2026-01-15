window.BENCHMARK_DATA = {
  "lastUpdate": 1768470395781,
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
        "date": 1768418560588,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_small_file",
            "value": 0.302632129038,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.311338629738,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read",
            "value": 0.301141302363,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 0.904588,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.31309502032200004,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.306881066272,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 0.84820425,
            "unit": "milliseconds"
          }
        ]
      },
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
        "date": 1768470395542,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_small_file",
            "value": 0.303723489136,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.31931533214999996,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read",
            "value": 0.312279598887,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 0.943222,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.322996139219,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.323720570264,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 0.6750435,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}