window.BENCHMARK_DATA = {
  "lastUpdate": 1719850396970,
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
        "date": 1719850396741,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10187786481966667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 69.29535389614766,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09059317921066667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.19216445911166669,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.11135344273433334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 71.979770072169,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16960209002866666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0405914942273333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 75.161075786706,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}