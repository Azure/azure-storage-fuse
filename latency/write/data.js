window.BENCHMARK_DATA = {
  "lastUpdate": 1711103797509,
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
          "id": "5883ec22f417b4d5fd9fd4c6499075aa349ca141",
          "message": "Add sudo to list and delete code",
          "timestamp": "2024-03-22T14:43:56+05:30",
          "tree_id": "9c816d00ef617f69ab0c306d7a0431f1e59f3953",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/5883ec22f417b4d5fd9fd4c6499075aa349ca141"
        },
        "date": 1711103797186,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 2.1666684528973335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 2.2364788027996667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 6.009125193498,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 16.157932965260667,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}