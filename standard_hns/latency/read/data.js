window.BENCHMARK_DATA = {
  "lastUpdate": 1719870967808,
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
        "date": 1719870967053,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.145995178965,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 142.96568768147867,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09965641885533333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.16811156858266665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.11347577159133333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 126.05545399395032,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17364679628966664,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0743435633783334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 130.45343153736835,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}