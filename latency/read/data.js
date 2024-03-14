window.BENCHMARK_DATA = {
  "lastUpdate": 1710403976296,
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
        "date": 1710403975961,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 1.8686329978346665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 69.45615940577767,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.3165726675796667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.8015380011153335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1.9462161813796666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 74.324787781524,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 2.3508007207996666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 4.220056138845,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_four_threads",
            "value": 75.688323472824,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}