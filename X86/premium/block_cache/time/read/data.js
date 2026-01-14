window.BENCHMARK_DATA = {
  "lastUpdate": 1768413376743,
  "repoUrl": "https://github.com/Azure/azure-storage-fuse",
  "entries": {
    "Benchmark": [
      {
        "commit": {
          "author": {
            "name": "syeleti-msft",
            "username": "syeleti-msft",
            "email": "syeleti@microsoft.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "cd2cb4303028ac433d46f7284e70869eb529a19e",
          "message": "Add parallel file writes to different files fio configs (#2096)",
          "timestamp": "2026-01-14T13:24:28Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/cd2cb4303028ac433d46f7284e70869eb529a19e"
        },
        "date": 1768403383490,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_small_file",
            "value": 0.29428238235799997,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.302576880297,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read",
            "value": 0.338286149385,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 74.828876548628,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.488988387676,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.386478117175,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 76.60037317474499,
            "unit": "milliseconds"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "syeleti-msft",
            "username": "syeleti-msft",
            "email": "syeleti@microsoft.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "cd2cb4303028ac433d46f7284e70869eb529a19e",
          "message": "Add parallel file writes to different files fio configs (#2096)",
          "timestamp": "2026-01-14T13:24:28Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/cd2cb4303028ac433d46f7284e70869eb529a19e"
        },
        "date": 1768413376496,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_small_file",
            "value": 0.244640161393,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.28975823082400004,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read",
            "value": 0.32245134521499996,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 74.260408064356,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.478946847451,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.330213639539,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 77.34342533548,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}