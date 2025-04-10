window.BENCHMARK_DATA = {
  "lastUpdate": 1744274600271,
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
        "date": 1741436789487,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 488.4915364583333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 371.8525390625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 533.1946614583334,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 534.478515625,
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
        "date": 1742382149609,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 2607.2574869791665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 1991.2470703125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2778.1875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2774.0875651041665,
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
        "date": 1744274600012,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 2669.8883463541665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 1926.0048828125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2594.3076171875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2661.5830078125,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}