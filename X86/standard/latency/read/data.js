window.BENCHMARK_DATA = {
  "lastUpdate": 1741632393253,
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
        "date": 1741426022487,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.096447993442,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 147.8490990167323,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.06309361017466668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18452272808466666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.11299851516600001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 153.69699595771735,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18121616616766667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0127051680446668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 149.43518789142732,
            "unit": "milliseconds"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "64532198+vibhansa-msft@users.noreply.github.com",
            "name": "Vikas Bhansali",
            "username": "vibhansa-msft"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "79978157a7cb7035566f743a8c86becadf2dec81",
          "message": "Merge branch 'main' into vibhansa/armperftest",
          "timestamp": "2025-03-10T22:32:05+05:30",
          "tree_id": "cbd7d68b0a780722eb7ff9ee8e431fec9495a607",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/79978157a7cb7035566f743a8c86becadf2dec81"
        },
        "date": 1741632393001,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.08606705954933334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 142.54389964454967,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.078415004612,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.1674058028583333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09791609649599999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 135.534866704028,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.181783134755,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9798878415543334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 139.35775692716498,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}