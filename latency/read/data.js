window.BENCHMARK_DATA = {
  "lastUpdate": 1709980606736,
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
          "id": "40062f15a812aab66612a083ca9219034c525917",
          "message": "Resolve syntax error",
          "timestamp": "2024-03-09T15:25:21+05:30",
          "tree_id": "090548bd0b938d63061a1985f6054304f5831ecc",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/40062f15a812aab66612a083ca9219034c525917"
        },
        "date": 1709980606406,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 1.8427507633066667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 73.70014888998833,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.3687249151943333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.7167867381806667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1.9438590716013333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 73.47347489097366,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 2.320588526123,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 4.460364812408333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_four_threads",
            "value": 74.058602850152,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}