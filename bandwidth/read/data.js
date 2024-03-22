window.BENCHMARK_DATA = {
  "lastUpdate": 1711095116548,
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
          "id": "3fc65e00d94fbcd0f92d0b476d203041e4bf4d6a",
          "message": "Correcting",
          "timestamp": "2024-03-22T12:56:15+05:30",
          "tree_id": "d44f911c12e9a097a1554f817ccb4ea7af784ed2",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/3fc65e00d94fbcd0f92d0b476d203041e4bf4d6a"
        },
        "date": 1711095115505,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 361.2532552083333,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 14.0126953125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2430.8645833333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1259.2620442708333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 363.7688802083333,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 14.485026041666666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 987.4381510416666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 2823.1302083333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 62.091796875,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}