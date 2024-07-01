window.BENCHMARK_DATA = {
  "lastUpdate": 1719855408722,
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
        "date": 1719855408473,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10368724437866667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 120.68809255898002,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07813544982833334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.14089965241466665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09418224095566667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 120.685692042494,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18912832308133334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0639820767493333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 122.379749094259,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}