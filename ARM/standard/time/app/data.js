window.BENCHMARK_DATA = {
  "lastUpdate": 1742390090523,
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
      },
      {
        "commit": {
          "author": {
            "email": "syeleti@microsoft.com",
            "name": "Srinivas Yeleti",
            "username": "syeleti-msft"
          },
          "committer": {
            "email": "syeleti@microsoft.com",
            "name": "Srinivas Yeleti",
            "username": "syeleti-msft"
          },
          "distinct": true,
          "id": "abeffac3531ff852a6240abfd960d263274a1527",
          "message": "remove warning errors that is preventing the run to happen",
          "timestamp": "2025-03-19T07:21:38Z",
          "tree_id": "fb855960d36d4e3f6914668bbc2c3a116cd23e8a",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/abeffac3531ff852a6240abfd960d263274a1527"
        },
        "date": 1742390090171,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.561169147491455,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 6.878725290298462,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 38.40088891983032,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 16.07281756401062,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.8480195999145508,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.74615740776062,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 33.055379152297974,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.368434190750122,
            "unit": "seconds"
          }
        ]
      }
    ]
  }
}