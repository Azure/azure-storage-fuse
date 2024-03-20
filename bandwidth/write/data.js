window.BENCHMARK_DATA = {
  "lastUpdate": 1710920853921,
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
          "id": "a34ec9f46def7c71da12fb55e24f370c72219ca4",
          "message": "Remove log printing stage",
          "timestamp": "2024-03-20T12:13:40+05:30",
          "tree_id": "4ea0b229bc95797534203bd46148d3d2b1608cae",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/a34ec9f46def7c71da12fb55e24f370c72219ca4"
        },
        "date": 1710920853496,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 1451.2985026041667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 675.681640625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_four_threads",
            "value": 1986.3382161458333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_four_threads",
            "value": 2016.0533854166667,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}