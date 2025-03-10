window.BENCHMARK_DATA = {
  "lastUpdate": 1741581606713,
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
          "id": "03e72e47d37985e5c28051c0ff17bdc0c7315e74",
          "message": "Correcting code for cache cleanup",
          "timestamp": "2025-03-07T23:48:22-08:00",
          "tree_id": "5b69276c81c0c728ae2dd3889b9743194fdcc990",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/03e72e47d37985e5c28051c0ff17bdc0c7315e74"
        },
        "date": 1741581606449,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.6046786308288574,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 7.068987607955933,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 38.962143659591675,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 17.734588861465454,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.7977554798126221,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 4.099169015884399,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 37.274746894836426,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 14.67838191986084,
            "unit": "seconds"
          }
        ]
      }
    ]
  }
}