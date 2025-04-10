window.BENCHMARK_DATA = {
  "lastUpdate": 1744275690639,
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
          "id": "03e72e47d37985e5c28051c0ff17bdc0c7315e74",
          "message": "Correcting code for cache cleanup",
          "timestamp": "2025-03-07T23:48:22-08:00",
          "tree_id": "5b69276c81c0c728ae2dd3889b9743194fdcc990",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/03e72e47d37985e5c28051c0ff17bdc0c7315e74"
        },
        "date": 1741438062127,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 110.0712890625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 120.0185546875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.107421875,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "syeleti@microsoft.com",
            "name": "Srinivas Yeleti",
            "username": "syeleti-msft"
          },
          "committer": {
            "email": "syeleti@microsoft.com",
            "name": "Srinivas Yeleti",
            "username": "syeleti-msft"
          },
          "distinct": true,
          "id": "abeffac3531ff852a6240abfd960d263274a1527",
          "message": "remove warning errors that is preventing the run to happen",
          "timestamp": "2025-03-19T07:21:38Z",
          "tree_id": "fb855960d36d4e3f6914668bbc2c3a116cd23e8a",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/abeffac3531ff852a6240abfd960d263274a1527"
        },
        "date": 1742383337967,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 133.3681640625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 135.9248046875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.1123046875,
            "unit": "MiB/s"
          }
        ]
      },
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
          "id": "643e91f6a0e3b6c677d4f89a36e8e5209e046ec6",
          "message": "Merge remote-tracking branch 'origin/main' into vibhansa/armperftest",
          "timestamp": "2025-04-09T21:55:32-07:00",
          "tree_id": "44e6262b9b6c585dac173204bc04e4eb084a6e47",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/643e91f6a0e3b6c677d4f89a36e8e5209e046ec6"
        },
        "date": 1744275690356,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 133.03125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 124.146484375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.1279296875,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}