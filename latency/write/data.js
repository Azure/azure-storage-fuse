window.BENCHMARK_DATA = {
  "lastUpdate": 1712248527051,
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
        "date": 1712229809479,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 0.12867097036666666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.13022197109833333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.14138339966866667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.12707977197400003,
            "unit": "milliseconds"
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
          "id": "af6c4b7f5027b190ed6fd22b9411c121cde95161",
          "message": "Sync with main",
          "timestamp": "2024-04-04T21:19:24+05:30",
          "tree_id": "8d44ebd434f1348fa4eccb4120623d585257ea2e",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/af6c4b7f5027b190ed6fd22b9411c121cde95161"
        },
        "date": 1712248526735,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 0.14480554085233333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.138369369974,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.13063489584766666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.13046343537533334,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}