window.BENCHMARK_DATA = {
  "lastUpdate": 1710434159614,
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
          "id": "c272d8b5b8f90fe9671356efd3e7587be836a870",
          "message": "Correct ioengine",
          "timestamp": "2024-03-14T21:30:19+05:30",
          "tree_id": "97809478e0f900cc621a2be24358a30eb6eef4f7",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/c272d8b5b8f90fe9671356efd3e7587be836a870"
        },
        "date": 1710434158439,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 519.2972005208334,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 13.709635416666666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2233.2470703125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1372.3759765625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 523.3974609375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 14.0322265625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 1584.2171223958333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 3708.9713541666665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_four_threads",
            "value": 54.895182291666664,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}