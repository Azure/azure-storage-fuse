window.BENCHMARK_DATA = {
  "lastUpdate": 1709709829297,
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
          "id": "fda6a3ce221e51de94abad16b0bbc5259c12caaf",
          "message": "Renaming config files to create order in graphs",
          "timestamp": "2024-03-06T12:18:31+05:30",
          "tree_id": "ee803bc621b695aa2bf7ac0a6382c58327f07583",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/fda6a3ce221e51de94abad16b0bbc5259c12caaf"
        },
        "date": 1709709828185,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 524.6767578125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 14.1591796875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2706.6982421875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1202.0146484375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 521.2815755208334,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 13.8046875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 1584.951171875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 3587.712890625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_four_threads",
            "value": 55.381510416666664,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}