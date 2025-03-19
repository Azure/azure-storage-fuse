window.BENCHMARK_DATA = {
  "lastUpdate": 1742387181264,
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
        "date": 1741577866004,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 106828.02197551087,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 65260.104923546045,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 110787.19100495208,
            "unit": "milliseconds"
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
        "date": 1742387180780,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 194.62624718550032,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 167.242904152828,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 9673.013667370002,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}