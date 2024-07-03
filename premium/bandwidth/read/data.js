window.BENCHMARK_DATA = {
  "lastUpdate": 1719980379293,
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
          "id": "65a677429517c28a75a8ab6a3051311f44ba96aa",
          "message": "Revert back to parallel runs",
          "timestamp": "2024-07-02T20:58:56-07:00",
          "tree_id": "6f63f586f70486b7a8ef5ccd4a64d7862cb7715c",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/65a677429517c28a75a8ab6a3051311f44ba96aa"
        },
        "date": 1719980378357,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2340.0094401041665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.4091796875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2443.9736328125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1316.0026041666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2286.0205078125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.43359375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4799.502604166667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4152.128580729167,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.2412109375,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}