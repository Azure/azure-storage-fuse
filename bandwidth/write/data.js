window.BENCHMARK_DATA = {
  "lastUpdate": 1711096857223,
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
        "date": 1711090086976,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 969.4521484375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 975.7916666666666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_four_threads",
            "value": 1188.1920572916667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_four_threads",
            "value": 1412.6741536458333,
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
        "date": 1711096856804,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 1025.9290364583333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 1018.482421875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2032.3317057291667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 1573.3059895833333,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}