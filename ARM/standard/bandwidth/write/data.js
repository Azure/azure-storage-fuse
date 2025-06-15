window.BENCHMARK_DATA = {
  "lastUpdate": 1749981585575,
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
        "date": 1744540400378,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 15650.454427083334,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write",
            "value": 2586.9130859375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 2083.8912760416665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2598.6516927083335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2629.8255208333335,
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
        "date": 1744634883495,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 18911.432942708332,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write",
            "value": 2687.0514322916665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 2024.4488932291667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2245.849609375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2639.4283854166665,
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
        "date": 1745142823960,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 17509.93359375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write",
            "value": 2615.4762369791665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 2024.3987630208333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2554.4635416666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2618.9788411458335,
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
        "date": 1745747429033,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 16846.560546875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write",
            "value": 2629.1048177083335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 2026.7845052083333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2728.044921875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2523.6015625,
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
        "date": 1746352825890,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 13648.332356770834,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write",
            "value": 2639.6337890625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 1925.9300130208333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2754.3798828125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2569.8274739583335,
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
        "date": 1746957488165,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 14986.857421875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write",
            "value": 2724.9264322916665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 2159.474609375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2700.845703125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2841.2630208333335,
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
        "date": 1747562232062,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 5513.117513020833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write",
            "value": 2755.3216145833335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 1958.1982421875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2708.5286458333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2652.4000651041665,
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
        "date": 1748167433458,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 14950.178059895834,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write",
            "value": 2686.4352213541665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 1986.2972005208333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2638.5198567708335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2556.9586588541665,
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
        "date": 1748772756862,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 13671.056640625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write",
            "value": 2567.7776692708335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 2063.5139973958335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2677.2522786458335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2710.208984375,
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
        "date": 1749376902530,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 14952.9833984375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write",
            "value": 2587.1686197916665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 1999.8134765625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2661.9358723958335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2635.2200520833335,
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
        "date": 1749981585313,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 13632.26171875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write",
            "value": 2688.2848307291665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 2044.044921875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 2693.1956380208335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2625.6650390625,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}