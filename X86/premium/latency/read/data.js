window.BENCHMARK_DATA = {
  "lastUpdate": 1741348716599,
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
          "id": "e531bd70068a4d7ecb9c2d1096f6cf78f31eadb7",
          "message": "Updated container name",
          "timestamp": "2025-03-07T03:39:29-08:00",
          "tree_id": "d61a69967fd61b62788c82e601611c72fb11db2a",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e531bd70068a4d7ecb9c2d1096f6cf78f31eadb7"
        },
        "date": 1741348716346,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10326116982166667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 74.89357459824134,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09189988627366667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18466226270466668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.11346633956633334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 73.269196885535,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18194483633033331,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0002579831616667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 74.77476939868033,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}