window.BENCHMARK_DATA = {
  "lastUpdate": 1767631365333,
  "repoUrl": "https://github.com/Azure/azure-storage-fuse",
  "entries": {
    "Benchmark": [
      {
        "commit": {
          "author": {
            "name": "Srinivas Yeleti",
            "username": "syeleti-msft",
            "email": "syeleti@microsoft.com"
          },
          "committer": {
            "name": "Srinivas Yeleti",
            "username": "syeleti-msft",
            "email": "syeleti@microsoft.com"
          },
          "id": "af160039bb50ceedeadb8e35d831a6d352de1395",
          "message": "modify benchmarks",
          "timestamp": "2026-01-05T09:12:19Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/af160039bb50ceedeadb8e35d831a6d352de1395"
        },
        "date": 1767631365077,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 1.314063975266,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 3.0198438764149995,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 1.1769774808523332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.26840024198133333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.8860980376266667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 0.8043208790729999,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.9328623465486666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0991545725403333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 0.868719349723,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}