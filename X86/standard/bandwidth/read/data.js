window.BENCHMARK_DATA = {
  "lastUpdate": 1766905499410,
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
          "id": "ba1d1eab212cda59fa94a4ca0f752f2b093ed7c5",
          "message": "Restore mount option'",
          "timestamp": "2025-04-10T04:02:03-07:00",
          "tree_id": "476165a49a1855d8214edc6bb7530574e73094a2",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/ba1d1eab212cda59fa94a4ca0f752f2b093ed7c5"
        },
        "date": 1744292547211,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2285.662109375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.6949869791666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2430.7379557291665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1448.0260416666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2267.6985677083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.7425130208333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 3769.7766927083335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3839.2447916666665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.058268229166667,
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
        "date": 1744345261654,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2342.7936197916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.720703125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2746.7630208333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1213.1813151041667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2369.1344401041665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.8304036458333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4743.591796875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3878.4677734375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.220052083333333,
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
          "id": "07c91329ece3d6310f1a56cdead7b10e449fc66f",
          "message": "Preload feature drop to main branch (#1678)\n\nCo-authored-by: Sourav Gupta <98318303+souravgupta-msft@users.noreply.github.com>\nCo-authored-by: souravgupta <souravgupta@microsoft.com>",
          "timestamp": "2025-04-11T09:17:56Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/07c91329ece3d6310f1a56cdead7b10e449fc66f"
        },
        "date": 1744525853254,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2341.0244140625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.8587239583333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2068.1917317708335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1327.0973307291667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2449.7513020833335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.9514973958333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4929.015950520833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3947.0611979166665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.5126953125,
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
          "id": "887bdba6cde3bc805787a410ea3fb4520a830392",
          "message": "Updating README for preload",
          "timestamp": "2025-04-13T23:47:01-07:00",
          "tree_id": "07eea2db5ae5f4e8cc51ae7445f1edbfbd81b581",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/887bdba6cde3bc805787a410ea3fb4520a830392"
        },
        "date": 1744620548117,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2305.0670572916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.8307291666666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2627.2503255208335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1499.046875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2239.9498697916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.9817708333333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4665.152669270833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3861.6520182291665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.81640625,
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
          "id": "07c91329ece3d6310f1a56cdead7b10e449fc66f",
          "message": "Preload feature drop to main branch (#1678)\n\nCo-authored-by: Sourav Gupta <98318303+souravgupta-msft@users.noreply.github.com>\nCo-authored-by: souravgupta <souravgupta@microsoft.com>",
          "timestamp": "2025-04-11T09:17:56Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/07c91329ece3d6310f1a56cdead7b10e449fc66f"
        },
        "date": 1745128800444,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2158.2080078125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.8167317708333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2935.6240234375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1641.9905598958333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2541.517578125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.9410807291666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4773.3544921875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4204.423828125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.611002604166667,
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
          "id": "43f48e9b789a9fc27d2138c4679ef8dc47cd55bf",
          "message": "Updating README for preload (#1685)\n\nCo-authored-by: Sourav Gupta <98318303+souravgupta-msft@users.noreply.github.com>",
          "timestamp": "2025-04-22T12:59:00+05:30",
          "tree_id": "97239a1303fad55d8f7adfa85b21e4ff6c56579d",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/43f48e9b789a9fc27d2138c4679ef8dc47cd55bf"
        },
        "date": 1745314248771,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2258.5966796875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.634765625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2474.3642578125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1443.8570963541667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2457.0621744791665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.8053385416666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4797.8046875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4054.0003255208335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.103841145833333,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "James Fantin-Hardesty",
            "username": "jfantinhardesty",
            "email": "24646452+jfantinhardesty@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "1667ad8b4bebf79badfccb915c351fd3209883a9",
          "message": "Feature: Lazy unmount (#1705)\n\nCo-authored-by: Vikas Bhansali <64532198+vibhansa-msft@users.noreply.github.com>",
          "timestamp": "2025-04-26T07:11:48Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/1667ad8b4bebf79badfccb915c351fd3209883a9"
        },
        "date": 1745733652408,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 1386.5690104166667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.3411458333333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2059.34765625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1280.654296875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2024.4713541666667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.4654947916666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4734.812825520833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3807.3938802083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 5.498372395833333,
            "unit": "MiB/s"
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
          "id": "4ddfcd4b776650ae5172663c04db2a0fb791cbd6",
          "message": "Fix logs using up all the space of /tmp folder (#1723)",
          "timestamp": "2025-05-03T08:50:17Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4ddfcd4b776650ae5172663c04db2a0fb791cbd6"
        },
        "date": 1746338701560,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2413.830078125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.8916015625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2860.6845703125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1649.3294270833333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2623.3203125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.9254557291666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4639.4296875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3441.6451822916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.2080078125,
            "unit": "MiB/s"
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
          "id": "4ddfcd4b776650ae5172663c04db2a0fb791cbd6",
          "message": "Fix logs using up all the space of /tmp folder (#1723)",
          "timestamp": "2025-05-03T08:50:17Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4ddfcd4b776650ae5172663c04db2a0fb791cbd6"
        },
        "date": 1746943428721,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2374.1481119791665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.748046875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 3075.6663411458335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1451.3955078125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2340.6907552083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.8232421875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4650.047200520833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3300.8684895833335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 6.867838541666667,
            "unit": "MiB/s"
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
          "id": "4ddfcd4b776650ae5172663c04db2a0fb791cbd6",
          "message": "Fix logs using up all the space of /tmp folder (#1723)",
          "timestamp": "2025-05-03T08:50:17Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4ddfcd4b776650ae5172663c04db2a0fb791cbd6"
        },
        "date": 1747548180467,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2474.9264322916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.7688802083333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 3116.3291015625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1275.8873697916667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2682.1376953125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.7005208333333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4675.015625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3329.3626302083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 6.550130208333333,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "ashruti-msft",
            "username": "ashruti-msft",
            "email": "137055338+ashruti-msft@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "5e0ecbfd0e63a70dca732601790538e455816b41",
          "message": "Mariner preview release location fix (#1785)",
          "timestamp": "2025-05-23T09:32:31Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/5e0ecbfd0e63a70dca732601790538e455816b41"
        },
        "date": 1748153077283,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2386.9339192708335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.7662760416666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2438.1748046875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1198.8541666666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2561.4365234375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.8639322916666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4594.862955729167,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3332.0911458333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.410807291666667,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "ashruti-msft",
            "username": "ashruti-msft",
            "email": "137055338+ashruti-msft@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "5e0ecbfd0e63a70dca732601790538e455816b41",
          "message": "Mariner preview release location fix (#1785)",
          "timestamp": "2025-05-23T09:32:31Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/5e0ecbfd0e63a70dca732601790538e455816b41"
        },
        "date": 1748758391446,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2248.9661458333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.6552734375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2452.5455729166665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1209.9567057291667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2478.0833333333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.7223307291666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4817.022786458333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3768.4339192708335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.006510416666667,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "ashruti-msft",
            "username": "ashruti-msft",
            "email": "137055338+ashruti-msft@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "5e0ecbfd0e63a70dca732601790538e455816b41",
          "message": "Mariner preview release location fix (#1785)",
          "timestamp": "2025-05-23T09:32:31Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/5e0ecbfd0e63a70dca732601790538e455816b41"
        },
        "date": 1749362763339,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2516.7350260416665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.7503255208333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2437.5569661458335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1443.4567057291667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2647.0944010416665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.783203125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4051.1845703125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3336.46484375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.107747395833333,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "ashruti-msft",
            "username": "ashruti-msft",
            "email": "137055338+ashruti-msft@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "5e0ecbfd0e63a70dca732601790538e455816b41",
          "message": "Mariner preview release location fix (#1785)",
          "timestamp": "2025-05-23T09:32:31Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/5e0ecbfd0e63a70dca732601790538e455816b41"
        },
        "date": 1749967628865,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2461.0286458333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.7796223958333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2669.7718098958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1284.806640625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2509.7906901041665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.82421875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4561.7041015625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3443.5266927083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.207682291666667,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "truealex81",
            "username": "truealex81",
            "email": "45783672+truealex81@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "8b5b9be10c43d6477ae33aa791c04c31537e3902",
          "message": "Update MIGRATION.md (#1837)",
          "timestamp": "2025-06-17T04:53:08Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/8b5b9be10c43d6477ae33aa791c04c31537e3902"
        },
        "date": 1750572681814,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2229.2171223958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.8375651041666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2798.8395182291665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1213.1588541666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2652.5888671875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.7545572916666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 5014.807291666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3521.6705729166665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.032552083333333,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "ashruti-msft",
            "username": "ashruti-msft",
            "email": "137055338+ashruti-msft@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "75a19ccf157f5497d79103bb0f99ddd55b4a5906",
          "message": "Ashruti/script fix (#1842)",
          "timestamp": "2025-06-24T10:29:37Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/75a19ccf157f5497d79103bb0f99ddd55b4a5906"
        },
        "date": 1751177930471,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2435.4905598958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.6643880208333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2476.9010416666665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1327.8304036458333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2561.7972005208335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.6728515625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4811.7099609375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3391.8837890625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 6.921875,
            "unit": "MiB/s"
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
          "id": "9fd527bf3c9d1c94ddb1f97083248208392e9fdb",
          "message": "fix rhel package installer in nightly pipeline (#1853)",
          "timestamp": "2025-07-04T12:07:45Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/9fd527bf3c9d1c94ddb1f97083248208392e9fdb"
        },
        "date": 1751782202429,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2590.9993489583335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.84375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2812.7360026041665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1144.0172526041667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2627.0843098958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.703125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4586.191080729167,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3750.3271484375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.085286458333333,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "Sourav Gupta",
            "username": "souravgupta-msft",
            "email": "98318303+souravgupta-msft@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "fa5351e70e3db2fddf8251d9b7d3e8b2b99fe4eb",
          "message": "Update PMC certificate (#1864)",
          "timestamp": "2025-07-09T11:19:04Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/fa5351e70e3db2fddf8251d9b7d3e8b2b99fe4eb"
        },
        "date": 1752387347915,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2435.8264973958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.7112630208333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2357.5735677083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1533.1318359375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2410.4248046875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.7981770833333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 5024.004231770833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3679.3570963541665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.279622395833333,
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
          "id": "05bb0853557011f6824b7738d633b063cf404bcc",
          "message": "Provide a mode to just disable kernel cache not the blobfuse cache (#1882)",
          "timestamp": "2025-07-17T13:55:25Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/05bb0853557011f6824b7738d633b063cf404bcc"
        },
        "date": 1752992186892,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2370.6591796875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.6363932291666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2944.884765625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1242.1471354166667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2609.7457682291665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.822265625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4784.382161458333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3801.392578125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.333658854166667,
            "unit": "MiB/s"
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
          "id": "7776998e3f031e791148482db29f8c40beb53255",
          "message": "Add New stage to the Nightly pipeline (#1889)",
          "timestamp": "2025-07-24T07:31:00Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/7776998e3f031e791148482db29f8c40beb53255"
        },
        "date": 1753596951240,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2323.8756510416665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.5107421875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2535.8343098958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1386.3505859375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2542.2509765625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.6702473958333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4660.655598958333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3269.4521484375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 6.661458333333333,
            "unit": "MiB/s"
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
          "id": "1a7d91ce786c8012c8afe26308e2c4d05fefd6aa",
          "message": "fix e2e tests failure  (#1913)\n\nCo-authored-by: Copilot <175728472+Copilot@users.noreply.github.com>",
          "timestamp": "2025-08-05T09:59:31Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/1a7d91ce786c8012c8afe26308e2c4d05fefd6aa"
        },
        "date": 1755411035226,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2400.8473307291665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.783203125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2735.625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1149.8860677083333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2507.9908854166665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.8479817708333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4396.9951171875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3568.8356119791665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.197265625,
            "unit": "MiB/s"
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
          "id": "1a7d91ce786c8012c8afe26308e2c4d05fefd6aa",
          "message": "fix e2e tests failure  (#1913)\n\nCo-authored-by: Copilot <175728472+Copilot@users.noreply.github.com>",
          "timestamp": "2025-08-05T09:59:31Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/1a7d91ce786c8012c8afe26308e2c4d05fefd6aa"
        },
        "date": 1756015576263,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2309.6422526041665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.734375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2442.259765625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1222.005859375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2647.9446614583335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.8681640625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4712.713216145833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3385.8291015625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.354817708333333,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "Copilot",
            "username": "Copilot",
            "email": "198982749+Copilot@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "ba815585e3ce3b2d08f0009de26c212e655af50c",
          "message": "Add comprehensive GitHub Copilot instructions for Azure Storage Fuse development (#1938)\n\nCo-authored-by: copilot-swe-agent[bot] <198982749+Copilot@users.noreply.github.com>\nCo-authored-by: vibhansa-msft <64532198+vibhansa-msft@users.noreply.github.com>",
          "timestamp": "2025-08-26T08:13:31Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/ba815585e3ce3b2d08f0009de26c212e655af50c"
        },
        "date": 1756620295485,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2427.3743489583335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.6708984375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2301.708984375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1252.9912109375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2451.240234375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.8069661458333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4674.712890625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3295.005859375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 6.734049479166667,
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
          "id": "dfa3e9d92d4849695965058de77c287f9a0901ce",
          "message": "AI Comment cleanup (#1995)",
          "timestamp": "2025-09-18T11:22:08Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/dfa3e9d92d4849695965058de77c287f9a0901ce"
        },
        "date": 1758434447986,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2231.3115234375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.6669921875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2704.1022135416665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1644.646484375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2613.3515625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.8092447916666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4725.223958333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3941.3291015625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.213216145833333,
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
          "id": "9bada825b18507d8648fb3d5a4271e8374f57978",
          "message": "Updating go dependencies (#1972)",
          "timestamp": "2025-09-26T09:30:51Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/9bada825b18507d8648fb3d5a4271e8374f57978"
        },
        "date": 1759039252747,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2151.5748697916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.8255208333333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 3206.6673177083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1270.2236328125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2623.7376302083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.8971354166666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 5195.287434895833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4075.337890625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.3974609375,
            "unit": "MiB/s"
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
          "id": "43314da664fe649d926fa148b6253ae28dff8d3f",
          "message": "Add FIO tests to check the data integrity (#1893)",
          "timestamp": "2025-09-29T10:20:04Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/43314da664fe649d926fa148b6253ae28dff8d3f"
        },
        "date": 1759644192410,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2373.4814453125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.8509114583333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2503.2942708333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1211.9921875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2700.4537760416665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.8662109375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4862.179036458333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3935.1611328125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.094075520833333,
            "unit": "MiB/s"
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
          "id": "389136cf285c96aae19b8e61c3c5bb0cee98bb45",
          "message": "Fix issues while truncating the file (#2003)\n\nCo-authored-by: Vikas Bhansali <64532198+vibhansa-msft@users.noreply.github.com>\nCo-authored-by: vibhansa <vibhansa@microsoft.com>",
          "timestamp": "2025-10-10T08:30:20Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/389136cf285c96aae19b8e61c3c5bb0cee98bb45"
        },
        "date": 1760096205102,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2332.3411458333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.703125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2806.9195963541665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1318.3470052083333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2577.0719401041665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.5836588541666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 5212.596354166667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3692.546875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.078776041666667,
            "unit": "MiB/s"
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
          "id": "389136cf285c96aae19b8e61c3c5bb0cee98bb45",
          "message": "Fix issues while truncating the file (#2003)\n\nCo-authored-by: Vikas Bhansali <64532198+vibhansa-msft@users.noreply.github.com>\nCo-authored-by: vibhansa <vibhansa@microsoft.com>",
          "timestamp": "2025-10-10T08:30:20Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/389136cf285c96aae19b8e61c3c5bb0cee98bb45"
        },
        "date": 1760251246902,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2397.0615234375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.6998697916666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2571.494140625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1234.8919270833333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2701.0065104166665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.8512369791666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 5057.123372395833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3867.328125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.361979166666667,
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
          "id": "4b679ae9a43c3d6ff4aa5d744280dc5ef3aa5849",
          "message": "Updating release date (#2032)",
          "timestamp": "2025-10-16T04:32:59Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4b679ae9a43c3d6ff4aa5d744280dc5ef3aa5849"
        },
        "date": 1760856216697,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2319.8388671875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.8857421875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2517.3987630208335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1648.01953125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2696.7649739583335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.9694010416666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4818.698567708333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4211.488606770833,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.419270833333333,
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
          "id": "4b679ae9a43c3d6ff4aa5d744280dc5ef3aa5849",
          "message": "Updating release date (#2032)",
          "timestamp": "2025-10-16T04:32:59Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4b679ae9a43c3d6ff4aa5d744280dc5ef3aa5849"
        },
        "date": 1761461389665,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2489.48046875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.7018229166666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2466.7281901041665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1465.9202473958333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2570.2555338541665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.9007161458333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 5138.962565104167,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4116.947591145833,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.1845703125,
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
          "id": "4b679ae9a43c3d6ff4aa5d744280dc5ef3aa5849",
          "message": "Updating release date (#2032)",
          "timestamp": "2025-10-16T04:32:59Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4b679ae9a43c3d6ff4aa5d744280dc5ef3aa5849"
        },
        "date": 1762065762249,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2534.3567708333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.6761067708333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2805.0589192708335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1186.9436848958333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2576.4055989583335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.9153645833333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4895.865885416667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4053.8759765625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.411458333333333,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "dependabot[bot]",
            "username": "dependabot[bot]",
            "email": "49699333+dependabot[bot]@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "421feb6dfe9ff7a89f7f224cb5af92f231539f18",
          "message": "Bump github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake from 1.4.2 to 1.4.3 (#2057)\n\nSigned-off-by: dependabot[bot] <support@github.com>\nCo-authored-by: dependabot[bot] <49699333+dependabot[bot]@users.noreply.github.com>",
          "timestamp": "2025-11-07T08:58:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/421feb6dfe9ff7a89f7f224cb5af92f231539f18"
        },
        "date": 1762670438704,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2209.6881510416665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.7555338541666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2681.8238932291665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1192.3375651041667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2642.4505208333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.8994140625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4833.77734375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4075.2854817708335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.37109375,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "Sourav Gupta",
            "username": "souravgupta-msft",
            "email": "98318303+souravgupta-msft@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "d2e3c8a69629afeda7e9b0d63074460fddbf8ca0",
          "message": "Adding mlperf scripts (#2061)",
          "timestamp": "2025-11-12T12:22:03Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/d2e3c8a69629afeda7e9b0d63074460fddbf8ca0"
        },
        "date": 1763275856752,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2236.1139322916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.802734375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2455.8395182291665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1286.4899088541667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2613.3037109375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.8102213541666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 5022.902994791667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3847.345703125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.3662109375,
            "unit": "MiB/s"
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
          "id": "f17c24583d29f72cf67eb652d27f482f87ecdc9f",
          "message": "Build support for arm32 (#2068)",
          "timestamp": "2025-11-21T09:53:19Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/f17c24583d29f72cf67eb652d27f482f87ecdc9f"
        },
        "date": 1763880622278,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2185.6930338541665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.7952473958333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2479.6168619791665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1287.6451822916667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2752.71875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.9475911458333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 5029.7666015625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3676.5305989583335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.2451171875,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "ashruti-msft",
            "username": "ashruti-msft",
            "email": "137055338+ashruti-msft@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "a0590abf380af29afc9f050df470afb0f8b0a251",
          "message": "Gen-config command improvement (#2067)",
          "timestamp": "2025-11-28T11:25:13Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/a0590abf380af29afc9f050df470afb0f8b0a251"
        },
        "date": 1764485550100,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2499.724609375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.6643880208333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2575.1217447916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1287.873046875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2603.0904947916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.7926432291666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 5142.928059895833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3722.6038411458335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.400065104166667,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "dependabot[bot]",
            "username": "dependabot[bot]",
            "email": "49699333+dependabot[bot]@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "bf0bb76533a5215eec5c79e4a6ffbef4d2024a77",
          "message": "Bump github.com/spf13/cobra from 1.10.1 to 1.10.2 (#2085)\n\nSigned-off-by: dependabot[bot] <support@github.com>\nCo-authored-by: dependabot[bot] <49699333+dependabot[bot]@users.noreply.github.com>",
          "timestamp": "2025-12-05T04:01:51Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/bf0bb76533a5215eec5c79e4a6ffbef4d2024a77"
        },
        "date": 1765090368779,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2295.2119140625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.8697916666666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2254.5657552083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1314.2389322916667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2699.4733072916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.9368489583333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4780.748697916667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3949.3391927083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.436848958333333,
            "unit": "MiB/s"
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
          "id": "d8a31a5066f5f064fdce5de9fbf44006bf0693d5",
          "message": "Fix linting issues (#2087)",
          "timestamp": "2025-12-12T08:13:45Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/d8a31a5066f5f064fdce5de9fbf44006bf0693d5"
        },
        "date": 1765695623553,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 1755.3942057291667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.5647786458333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2412.9300130208335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1246.0608723958333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2563.8167317708335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.765625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 5138.627604166667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4023.9361979166665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.035807291666667,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "Sourav Gupta",
            "username": "souravgupta-msft",
            "email": "98318303+souravgupta-msft@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "dd6a9cf285ebcefc98cdc8ebc7405b889ba4c65e",
          "message": "Add goroutine id in debug logs (#2063)",
          "timestamp": "2025-12-17T09:18:20Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/dd6a9cf285ebcefc98cdc8ebc7405b889ba4c65e"
        },
        "date": 1766300547345,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2227.125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.8919270833333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2694.6266276041665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1199.431640625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2644.064453125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 2.0276692708333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4868.289713541667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4199.757161458333,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.612955729166667,
            "unit": "MiB/s"
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
          "id": "687ac7f12b8f119ff944acba16c1439838d8932e",
          "message": "Refactor tests (#2090)",
          "timestamp": "2025-12-24T09:11:18Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/687ac7f12b8f119ff944acba16c1439838d8932e"
        },
        "date": 1766905498036,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2456.1315104166665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.859375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2458.4759114583335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1348.7936197916667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2650.578125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 2.0035807291666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 5078.358723958333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3702.7327473958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 7.6298828125,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}