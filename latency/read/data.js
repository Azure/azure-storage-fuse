window.BENCHMARK_DATA = {
  "lastUpdate": 1710337061821,
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
          "id": "745533b77e66116f53b500f960152562b69910e5",
          "message": "restart with fresh data",
          "timestamp": "2024-03-13T18:34:49+05:30",
          "tree_id": "a6e4530290bd04a32a7ab40e8a86830558ef55c6",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/745533b77e66116f53b500f960152562b69910e5"
        },
        "date": 1710337061493,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.40578348940433334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 40.55379578452567,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.425724747347,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.741858575526,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.3789914878376666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 39.271154486583335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 0.7454769340443334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 4.255090004776334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_four_threads",
            "value": 2.179705404854,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}