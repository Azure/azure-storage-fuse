window.BENCHMARK_DATA = {
  "lastUpdate": 1712231533559,
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
        "date": 1712231533179,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "list_100k_files",
            "value": "12.78000",
            "unit": "seconds"
          },
          {
            "name": "delete_100k_files",
            "value": "382.49000",
            "unit": "seconds"
          }
        ]
      }
    ]
  }
}