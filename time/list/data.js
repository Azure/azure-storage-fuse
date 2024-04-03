window.BENCHMARK_DATA = {
  "lastUpdate": 1712143286402,
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
          "id": "40dc943aaeb620f4c1bff6497796f272d091b109",
          "message": "Correct list and del output",
          "timestamp": "2024-03-26T21:55:09+05:30",
          "tree_id": "8be23de9488d8b0c1915d7cd89a304cdeafc44da",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/40dc943aaeb620f4c1bff6497796f272d091b109"
        },
        "date": 1711476109420,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "list_100k_files",
            "value": "20.43000",
            "unit": "seconds"
          },
          {
            "name": "delete_100k_files",
            "value": "398.62000",
            "unit": "seconds"
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
          "id": "482a82eb5445945508713706c9768c7a442d8c88",
          "message": "Merge remote-tracking branch 'origin/main' into vibhansa/perftestrunner",
          "timestamp": "2024-04-03T13:58:04+05:30",
          "tree_id": "c4074fe168e90d751e699de98801bf2169e6aac8",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/482a82eb5445945508713706c9768c7a442d8c88"
        },
        "date": 1712143286036,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "list_100k_files",
            "value": "10.96000",
            "unit": "seconds"
          },
          {
            "name": "delete_100k_files",
            "value": "393.83000",
            "unit": "seconds"
          }
        ]
      }
    ]
  }
}