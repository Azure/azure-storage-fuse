window.BENCHMARK_DATA = {
  "lastUpdate": 1746953907869,
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
        "date": 1741438727898,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 0.9445252418518066,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 3.836538076400757,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 37.9499990940094,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 15.608469247817993,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.8138916492462158,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.811798095703125,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 37.514626026153564,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 14.37084698677063,
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
        "date": 1742384045262,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 0.8020122051239014,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 4.315343141555786,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 37.53566288948059,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 16.310912609100342,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.743694543838501,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.515763759613037,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 33.495232343673706,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.25979495048523,
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
        "date": 1744276372221,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.1538622379302979,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 5.166339874267578,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 39.74119853973389,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 14.783462524414062,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.8571562767028809,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.4552149772644043,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 34.0309739112854,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.53883981704712,
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
          "id": "ba1d1eab212cda59fa94a4ca0f752f2b093ed7c5",
          "message": "Restore mount option'",
          "timestamp": "2025-04-10T04:02:03-07:00",
          "tree_id": "476165a49a1855d8214edc6bb7530574e73094a2",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/ba1d1eab212cda59fa94a4ca0f752f2b093ed7c5"
        },
        "date": 1744303017356,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.1065244674682617,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 4.118869066238403,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 39.96237897872925,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 15.46134352684021,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.6379764080047607,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.4582133293151855,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 33.693076372146606,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.353879451751709,
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
        "date": 1744355513529,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 0.8896656036376953,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 4.077868700027466,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 37.574944496154785,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 15.700565338134766,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.732691764831543,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.349646806716919,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 32.546515464782715,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 12.951375484466553,
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
        "date": 1744536624907,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.1182589530944824,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 4.872279405593872,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 39.63785672187805,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 15.723344087600708,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.8377861976623535,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.3523778915405273,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 32.68408441543579,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 12.78471302986145,
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
          "id": "887bdba6cde3bc805787a410ea3fb4520a830392",
          "message": "Updating README for preload",
          "timestamp": "2025-04-13T23:47:01-07:00",
          "tree_id": "07eea2db5ae5f4e8cc51ae7445f1edbfbd81b581",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/887bdba6cde3bc805787a410ea3fb4520a830392"
        },
        "date": 1744631271808,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 0.9789061546325684,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 4.703309774398804,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 39.20243835449219,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 15.666927337646484,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.7593274116516113,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.4208567142486572,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 33.560039043426514,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.20144271850586,
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
        "date": 1745139252203,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.0852160453796387,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 4.00786828994751,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 40.75853705406189,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 15.188091516494751,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.8622658252716064,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.374156951904297,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 32.135979413986206,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.000917911529541,
            "unit": "seconds"
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
        "date": 1745743880075,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 0.878262996673584,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 4.015458583831787,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 37.17105221748352,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 15.590733528137207,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.7337596416473389,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.252476692199707,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 32.973763942718506,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 12.739840269088745,
            "unit": "seconds"
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
        "date": 1746349242133,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.0261039733886719,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 4.0485734939575195,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 38.88219141960144,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 15.567113399505615,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.6813778877258301,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.317044258117676,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 32.20417785644531,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 12.975130319595337,
            "unit": "seconds"
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
        "date": 1746953907641,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 0.9314794540405273,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 4.279787540435791,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 37.75177454948425,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 14.913963079452515,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.78179931640625,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.341601848602295,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 39.43041133880615,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.039489269256592,
            "unit": "seconds"
          }
        ]
      }
    ]
  }
}