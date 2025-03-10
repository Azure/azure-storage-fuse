window.BENCHMARK_DATA = {
  "lastUpdate": 1741638531421,
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
        "date": 1741432079185,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2715.4977213541665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.998046875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2521.4124348958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1375.5895182291667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2456.2457682291665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 4.345052083333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 6722.8935546875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 6985.203450520833,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 15.16796875,
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
        "date": 1741638530137,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2751.2718098958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.6852213541666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2547.3352864583335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1397.7529296875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2320.5221354166665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.8567708333333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 6665.963541666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 6988.333333333333,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.941731770833334,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}