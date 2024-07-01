window.BENCHMARK_DATA = {
  "lastUpdate": 1719855407576,
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
        "date": 1719855407324,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2190.9404296875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 2.0716145833333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2724.2503255208335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1603.7991536458333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2646.2223307291665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 2.0709635416666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4526.586263020833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3649.8427734375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 8.161458333333334,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}