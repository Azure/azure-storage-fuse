window.BENCHMARK_DATA = {
  "lastUpdate": 1710922697455,
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
          "id": "3bc15cf143c5c8ccfbe50dce3f4ee190c9a6fe02",
          "message": "Reset open files setting",
          "timestamp": "2024-03-19T15:31:59+05:30",
          "tree_id": "bcd3da2f495e1f5e3719d8bae7fb32b00f9ec479",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/3bc15cf143c5c8ccfbe50dce3f4ee190c9a6fe02"
        },
        "date": 1710844290241,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 120.2353515625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 115.5263671875,
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
          "id": "a34ec9f46def7c71da12fb55e24f370c72219ca4",
          "message": "Remove log printing stage",
          "timestamp": "2024-03-20T12:13:40+05:30",
          "tree_id": "4ea0b229bc95797534203bd46148d3d2b1608cae",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/a34ec9f46def7c71da12fb55e24f370c72219ca4"
        },
        "date": 1710922697118,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 94.330078125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 98,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.0908203125,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}