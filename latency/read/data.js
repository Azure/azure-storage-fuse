window.BENCHMARK_DATA = {
  "lastUpdate": 1711088206980,
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
        "date": 1711088206666,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2.65825830176,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 74.82691684969467,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.365679316756,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.632651547937,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2.6130325868293336,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 71.70171070567402,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 3.9234145113523335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 5.513621686000666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_four_threads",
            "value": 66.44860537647968,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}