window.BENCHMARK_DATA = {
  "lastUpdate": 1719485215942,
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
          "id": "eb76cd1603914937829d87512f5ae46f03bb139d",
          "message": "Updated",
          "timestamp": "2024-06-27T03:26:05-07:00",
          "tree_id": "59cb8c935adfa5c3dc8d51a5df4ae9fed760a198",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/eb76cd1603914937829d87512f5ae46f03bb139d"
        },
        "date": 1719485215715,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09934113540899998,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 124.04179716118533,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.06285380940466666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18251558277533333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10436621564466668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 119.50039470376066,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.167294470708,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0764072377776666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 120.80521805983233,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}