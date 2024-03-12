window.BENCHMARK_DATA = {
  "lastUpdate": 1710231043437,
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
      },
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
          "id": "c09710ac06d604a432283cdcc3fd4aabef423b4c",
          "message": "comment out cpu usage graphs",
          "timestamp": "2024-03-12T10:44:41+05:30",
          "tree_id": "56c78cae1570831b72f6ec83a71381f37d5103fe",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/c09710ac06d604a432283cdcc3fd4aabef423b4c"
        },
        "date": 1710222606201,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 1.8958245621353333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 73.20274464868932,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.3159397455286666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.713034870813,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1.9092504982299998,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 74.47513750837933,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 2.3084060536366664,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 4.464020068845667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_four_threads",
            "value": 73.033571281453,
            "unit": "milliseconds"
          }
        ]
      },
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
          "id": "1b0361d83ff9020be035865da7cd140f8399c93a",
          "message": "adding local cache in all tests",
          "timestamp": "2024-03-12T13:08:20+05:30",
          "tree_id": "05642bb2cdcf7cd598c97c9fe4ad52d2071b8c28",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/1b0361d83ff9020be035865da7cd140f8399c93a"
        },
        "date": 1710231043102,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.40995195772966664,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 41.20955670128733,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.44473631897,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.789185786959,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.407856899503,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 28.721952297614333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 0.7177080716276666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 4.309899979736667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_four_threads",
            "value": 2.2151115348743335,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}