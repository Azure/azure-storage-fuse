window.BENCHMARK_DATA = {
  "lastUpdate": 1710337863230,
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
        "date": 1710337862885,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 5.147503207635,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 5.311213555147334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_four_threads",
            "value": 1.1776043633976667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_four_threads",
            "value": 3.6405613970106665,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}