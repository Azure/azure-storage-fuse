window.BENCHMARK_DATA = {
  "lastUpdate": 1719870965987,
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
        "date": 1719870965678,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 1643.9694010416667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.7604166666666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2204.4287109375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1401.6526692708333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2212.197265625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.9827473958333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4833.594075520833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3640.2513020833335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.654296875,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}