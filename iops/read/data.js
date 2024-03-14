window.BENCHMARK_DATA = {
  "lastUpdate": 1710403977431,
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
        "date": 1710403977113,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 520.820842,
            "unit": "iops"
          },
          {
            "name": "random_read",
            "value": 14.406905333333333,
            "unit": "iops"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2738.5642700000003,
            "unit": "iops"
          },
          {
            "name": "random_read_small_file",
            "value": 1170.0609983333334,
            "unit": "iops"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 513.8063836666666,
            "unit": "iops"
          },
          {
            "name": "random_read_direct_io",
            "value": 13.455557666666666,
            "unit": "iops"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 1575.9850996666667,
            "unit": "iops"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 3696.143922333333,
            "unit": "iops"
          },
          {
            "name": "random_read_four_threads",
            "value": 52.60234466666666,
            "unit": "iops"
          }
        ]
      }
    ]
  }
}