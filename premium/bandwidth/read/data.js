window.BENCHMARK_DATA = {
  "lastUpdate": 1719850395885,
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
        "date": 1719850389508,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2229.0224609375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.607421875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2376.1028645833335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1206.6149088541667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2239.2662760416665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.47265625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4850.0595703125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3717.5406901041665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.275390625,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}