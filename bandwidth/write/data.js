window.BENCHMARK_DATA = {
  "lastUpdate": 1712229808684,
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
          "id": "1a4e554337ce8799974951b862bc67522031adf1",
          "message": "Correcting bs in large write case",
          "timestamp": "2024-04-04T15:58:04+05:30",
          "tree_id": "4a43bfe9042ae83dab8725a2ba1ea42ed150b950",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/1a4e554337ce8799974951b862bc67522031adf1"
        },
        "date": 1712229808328,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 1944.7877604166667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 1919.0882161458333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 1783.4928385416667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 1962.7897135416667,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}