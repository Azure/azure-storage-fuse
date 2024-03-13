window.BENCHMARK_DATA = {
  "lastUpdate": 1710337861941,
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
          "id": "745533b77e66116f53b500f960152562b69910e5",
          "message": "restart with fresh data",
          "timestamp": "2024-03-13T18:34:49+05:30",
          "tree_id": "a6e4530290bd04a32a7ab40e8a86830558ef55c6",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/745533b77e66116f53b500f960152562b69910e5"
        },
        "date": 1710337861589,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 194.197265625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 188.15234375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_four_threads",
            "value": 2671.1419270833335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_four_threads",
            "value": 4318.680989583333,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}