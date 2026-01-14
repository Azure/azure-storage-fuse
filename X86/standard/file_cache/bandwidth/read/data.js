window.BENCHMARK_DATA = {
  "lastUpdate": 1768400592004,
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
        "date": 1768400590492,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_small_file",
            "value": 3251.2578125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 3061.564453125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read",
            "value": 797.85546875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 0.0107421875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 11328.6865234375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 43070.4521484375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 0.04296875,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}