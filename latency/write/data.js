window.BENCHMARK_DATA = {
  "lastUpdate": 1711096858352,
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
        "date": 1711090088242,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 2.2590435732423333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 2.167258183659,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_four_threads",
            "value": 4.987543459928333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_four_threads",
            "value": 11.883102555169666,
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
          "id": "3fc65e00d94fbcd0f92d0b476d203041e4bf4d6a",
          "message": "Correcting",
          "timestamp": "2024-03-22T12:56:15+05:30",
          "tree_id": "d44f911c12e9a097a1554f817ccb4ea7af784ed2",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/3fc65e00d94fbcd0f92d0b476d203041e4bf4d6a"
        },
        "date": 1711096858047,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 2.2098576756183332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 2.1664285974286663,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 1.8984424703043334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 10.687160101475001,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}