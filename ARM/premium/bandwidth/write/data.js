window.BENCHMARK_DATA = {
  "lastUpdate": 1749371206772,
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
        "date": 1744300818320,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 2757.6936848958335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 1944.8932291666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2774.1930338541665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2723.6975911458335,
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
        "date": 1744353377893,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 2754.2018229166665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 2085.65234375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2694.2763671875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2722.3310546875,
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
        "date": 1744534277477,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 13046.776041666666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write",
            "value": 2656.6497395833335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 2039.6927083333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2824.9635416666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2732.8955078125,
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
        "date": 1744628923859,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 14942.7119140625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write",
            "value": 2745.474609375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 2069.81640625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2724.8030598958335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2752.8079427083335,
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
        "date": 1745136872861,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 16803.395833333332,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write",
            "value": 2712.623046875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 2058.3727213541665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2681.9404296875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2830.1002604166665,
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
        "date": 1745741521847,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 12625.013671875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write",
            "value": 2721.4436848958335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 2115.865234375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2735.4254557291665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2556.1946614583335,
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
        "date": 1746346952704,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 14933.3330078125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write",
            "value": 2688.1722005208335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 1976.6946614583333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2787.9899088541665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2674.4772135416665,
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
        "date": 1746951817244,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 16129.7646484375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write",
            "value": 2740.9163411458335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 2067.0556640625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2797.8147786458335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2726.5084635416665,
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
        "date": 1747556408182,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 14941.496744791666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write",
            "value": 2787.5094401041665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 1915.1468098958333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2767.1376953125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2738.6832682291665,
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
        "date": 1748161541845,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 14940.897786458334,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write",
            "value": 2665.5719401041665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 1979.6901041666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2756.6184895833335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2659.2994791666665,
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
        "date": 1748766951084,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 15053.977213541666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write",
            "value": 2738.1279296875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 1939.5537109375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2527.7151692708335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2702.359375,
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
        "date": 1749371206471,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 13635.2158203125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write",
            "value": 2716.9368489583335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 1974.0833333333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2789.2373046875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2698.412109375,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}