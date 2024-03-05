window.BENCHMARK_DATA = {
  "lastUpdate": 1709635287429,
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
          "id": "24f92f91e15a8a56fc84c41bd5b29390c91ca696",
          "message": "Files renamed",
          "timestamp": "2024-03-05T15:35:46+05:30",
          "tree_id": "7ea9abff5f30f29c6b946340d57f1aee8b23025c",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/24f92f91e15a8a56fc84c41bd5b29390c91ca696"
        },
        "date": 1709635286336,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "random_read_four_threads",
            "value": 56.01953125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 14.555013020833334,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1337.6656901041667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 12.595052083333334,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 3629.8157552083335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 1544.1650390625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 503.2936197916667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2362.1875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read",
            "value": 522.0524088541666,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}