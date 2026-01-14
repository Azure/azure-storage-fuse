window.BENCHMARK_DATA = {
  "lastUpdate": 1768403382259,
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
        "date": 1768403379628,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_small_file",
            "value": 3394.353515625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 3299.15625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read",
            "value": 2953.220703125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 13.361328125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 7899.4052734375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 9158.1884765625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 52.1845703125,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}