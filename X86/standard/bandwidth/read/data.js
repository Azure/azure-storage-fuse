window.BENCHMARK_DATA = {
  "lastUpdate": 1744267120937,
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
        "date": 1741426020521,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2335.1077473958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.6907552083333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 3227.5419921875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1252.4521484375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2206.4817708333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.6272786458333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4597.584635416667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3836.5074869791665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 6.67578125,
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
        "date": 1741632390849,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2586.02734375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.7532552083333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2726.6891276041665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1392.3505859375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2555.5198567708335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.8440755208333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4583.993815104167,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3989.9114583333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.2041015625,
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
        "date": 1742375060094,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2025.943359375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.3219401041666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2465.373046875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1640.5725911458333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2421.0100911458335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.490234375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4814.325520833333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3710.5240885416665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 5.787760416666667,
            "unit": "MiB/s"
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
          "id": "643e91f6a0e3b6c677d4f89a36e8e5209e046ec6",
          "message": "Merge remote-tracking branch 'origin/main' into vibhansa/armperftest",
          "timestamp": "2025-04-09T21:55:32-07:00",
          "tree_id": "44e6262b9b6c585dac173204bc04e4eb084a6e47",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/643e91f6a0e3b6c677d4f89a36e8e5209e046ec6"
        },
        "date": 1744267119794,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2235.9498697916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.7919921875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2429.251953125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1152.7080078125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2271.9485677083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.8766276041666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4867.149088541667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3934.7555338541665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.318033854166667,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}