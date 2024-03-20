window.BENCHMARK_DATA = {
  "lastUpdate": 1710920855119,
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
        "date": 1710920854801,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 0.6181185440710001,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 2.7154541871609994,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_four_threads",
            "value": 1.9413566490926666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_four_threads",
            "value": 7.957378621574001,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}