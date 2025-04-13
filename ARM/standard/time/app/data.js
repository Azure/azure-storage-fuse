window.BENCHMARK_DATA = {
  "lastUpdate": 1744543243182,
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
        "date": 1741581606449,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.6046786308288574,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 7.068987607955933,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 38.962143659591675,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 17.734588861465454,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.7977554798126221,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 4.099169015884399,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 37.274746894836426,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 14.67838191986084,
            "unit": "seconds"
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
        "date": 1742390090171,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.561169147491455,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 6.878725290298462,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 38.40088891983032,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 16.07281756401062,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.8480195999145508,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.74615740776062,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 33.055379152297974,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.368434190750122,
            "unit": "seconds"
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
        "date": 1744283408436,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.4929091930389404,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 6.485959768295288,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 45.667165756225586,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 17.508986949920654,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.729830265045166,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 2.8371331691741943,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 26.010287761688232,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 8.660654544830322,
            "unit": "seconds"
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
        "date": 1744361859405,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.3770718574523926,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 4.594048976898193,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 38.6491961479187,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 15.679591417312622,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 1.378108263015747,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.6384198665618896,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 33.187501668930054,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.028192520141602,
            "unit": "seconds"
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
          "id": "07c91329ece3d6310f1a56cdead7b10e449fc66f",
          "message": "Preload feature drop to main branch (#1678)\n\nCo-authored-by: Sourav Gupta <98318303+souravgupta-msft@users.noreply.github.com>\nCo-authored-by: souravgupta <souravgupta@microsoft.com>",
          "timestamp": "2025-04-11T09:17:56Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/07c91329ece3d6310f1a56cdead7b10e449fc66f"
        },
        "date": 1744543242943,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.2622008323669434,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 6.110636234283447,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 38.8404221534729,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 15.216640949249268,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.806145191192627,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.7221200466156006,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 33.11217451095581,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 12.932368755340576,
            "unit": "seconds"
          }
        ]
      }
    ]
  }
}