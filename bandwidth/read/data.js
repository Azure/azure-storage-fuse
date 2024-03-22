window.BENCHMARK_DATA = {
  "lastUpdate": 1711088205766,
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
        "date": 1711088204706,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 368.3662109375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 13.358723958333334,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2379.5319010416665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1454.2252604166667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 382.6136067708333,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 13.972330729166666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 972.19140625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 2797.6282552083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_four_threads",
            "value": 59.887369791666664,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}