window.BENCHMARK_DATA = {
  "lastUpdate": 1710337060646,
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
        "date": 1710337059367,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2191.3157552083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 24.653971354166668,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2073.763671875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1281.7125651041667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2637.1780598958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 25.644205729166668,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 4181.7548828125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 3762.1634114583335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_four_threads",
            "value": 1703.7317708333333,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}