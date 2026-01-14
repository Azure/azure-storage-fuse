window.BENCHMARK_DATA = {
  "lastUpdate": 1768399255980,
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
        "date": 1768399255728,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 0.547178563789,
            "unit": "milliseconds"
          },
          {
            "name": "seq_write_parallel_16_files",
            "value": 2.808882537885,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}