window.BENCHMARK_DATA = {
  "lastUpdate": 1768414817099,
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
        "date": 1768404926418,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_small_file",
            "value": 0.324758762181,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.317387668793,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read",
            "value": 0.37099805456999996,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 1.134316,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.372306986602,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.371767458582,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 1.45094325,
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
        "date": 1768414816844,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_small_file",
            "value": 0.38195189162200005,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.24116892184,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read",
            "value": 0.38203240282199996,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 1.304452,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.37309165542,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.368152411853,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 1.0763815,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}