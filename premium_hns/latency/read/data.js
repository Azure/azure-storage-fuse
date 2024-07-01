window.BENCHMARK_DATA = {
  "lastUpdate": 1719860631976,
  "repoUrl": "https://github.com/Azure/azure-storage-fuse",
  "entries": {
    "Benchmark": [
      {
        "commit": {
          "author": {
            "email": "vibhansa@microsoft.com",
            "name": "vibhansa",
            "username": "vibhansa-msft"
          },
          "committer": {
            "email": "vibhansa@microsoft.com",
            "name": "vibhansa",
            "username": "vibhansa-msft"
          },
          "distinct": true,
          "id": "233235c3c0d20b6a63516920f18cb331c3f07302",
          "message": "updated",
          "timestamp": "2024-07-01T04:09:13-07:00",
          "tree_id": "ee1d47c48275271aa297e35d238317578db6cbb2",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/233235c3c0d20b6a63516920f18cb331c3f07302"
        },
        "date": 1719860631719,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.08818861721366666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 66.61404364868501,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09193157417500002,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.19101433868533335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.11162358425133334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 66.49034697609233,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.1878709350176667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0309954510206667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 73.59087217117833,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}