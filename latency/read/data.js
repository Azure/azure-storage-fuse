window.BENCHMARK_DATA = {
  "lastUpdate": 1711095117731,
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
        "date": 1711095117397,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2.7106584975836667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 71.433640807639,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.35992511847866665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.7412106935446667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2.749602134887,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 69.074414944248,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 3.8769015461786664,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 5.429224681808667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 64.05462198880267,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}