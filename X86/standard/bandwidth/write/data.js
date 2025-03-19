window.BENCHMARK_DATA = {
  "lastUpdate": 1742376359069,
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
        "date": 1741427855161,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 1792.166015625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 1895.0309244791667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 1922.1910807291667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 1994.0445963541667,
            "unit": "MiB/s"
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
        "date": 1741634186238,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 1879.4534505208333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 1764.8987630208333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 1820.2395833333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 1985.7776692708333,
            "unit": "MiB/s"
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
        "date": 1742376358806,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 1886.583984375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 1681.1409505208333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2029.5237630208333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 1980.4641927083333,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}