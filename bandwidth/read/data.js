window.BENCHMARK_DATA = {
  "lastUpdate": 1710403975076,
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
          "id": "5dd24784209e38dc1c002332394aa6a3646d1f8f",
          "message": "Mount highly parallel with disk cache",
          "timestamp": "2024-03-14T13:10:06+05:30",
          "tree_id": "d3cb0cfdd25bad68bd3830c79148b6da6106ab4e",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/5dd24784209e38dc1c002332394aa6a3646d1f8f"
        },
        "date": 1710403974027,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 520.8203125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 14.406575520833334,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2738.5638020833335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1170.060546875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 513.8063151041666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 13.455078125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 1575.9847005208333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 3696.1435546875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_four_threads",
            "value": 52.601888020833336,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}