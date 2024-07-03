window.BENCHMARK_DATA = {
  "lastUpdate": 1719985583649,
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
        "date": 1719985583419,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.08566508638866667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 124.51619898046734,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09684036961999999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18363040106466666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09229068084233333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 119.21420996782501,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.1772007433103333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1698711301883333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 121.655145878031,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}