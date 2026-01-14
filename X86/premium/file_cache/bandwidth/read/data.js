window.BENCHMARK_DATA = {
  "lastUpdate": 1768404925165,
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
        "date": 1768404924763,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_small_file",
            "value": 3076.4638671875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 3145.76171875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read",
            "value": 739.0244140625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 0.009765625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 10734.876953125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 42935.009765625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 0.0400390625,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}