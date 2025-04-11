window.BENCHMARK_DATA = {
  "lastUpdate": 1744359006495,
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
        "date": 1741579042478,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 408.7867838541667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 325.1201171875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 502.2581380208333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 463.1149088541667,
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
        "date": 1742387660769,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 2617.576171875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 2026.6461588541667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2666.9938151041665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2704.2454427083335,
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
        "date": 1744279959519,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 2125.9814453125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 2101.7490234375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2087.4108072916665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2093.4938151041665,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "Vikas Bhansali",
            "username": "vibhansa-msft",
            "email": "64532198+vibhansa-msft@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "0cd7596da267f6a7f6ad7cd9126fc9ae1305d3bd",
          "message": "Adding arm based benchmark tests (#1654)\n\nCo-authored-by: Srinivas Yeleti <syeleti@microsoft.com>",
          "timestamp": "2025-04-10T15:43:25Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/0cd7596da267f6a7f6ad7cd9126fc9ae1305d3bd"
        },
        "date": 1744359006246,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 2690.4348958333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 2091.07421875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2404.7916666666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2642.7945963541665,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}