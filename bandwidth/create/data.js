window.BENCHMARK_DATA = {
  "lastUpdate": 1711091878764,
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
          "id": "3b94d46553db8532947204be7097a74522395641",
          "message": "Add logs",
          "timestamp": "2024-03-22T11:00:48+05:30",
          "tree_id": "536af633e36721ecf2ad7f860a616fd543eb5d01",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/3b94d46553db8532947204be7097a74522395641"
        },
        "date": 1711091878452,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 98.6962890625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 99.2548828125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.091796875,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}