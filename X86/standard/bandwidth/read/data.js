window.BENCHMARK_DATA = {
  "lastUpdate": 1755411036588,
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
      }
    ]
  }
}