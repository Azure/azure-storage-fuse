window.BENCHMARK_DATA = {
  "lastUpdate": 1763891571902,
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
        "date": 1741435535736,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 29458.78335417612,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 63288.68052552369,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 51545.48760337033,
            "unit": "milliseconds"
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
        "date": 1742381677329,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 161.148709476672,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 155.09053181132865,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 7808.237296469354,
            "unit": "milliseconds"
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
        "date": 1744274127273,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 160.2302060952437,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 173.49095104637203,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 7778.661673407921,
            "unit": "milliseconds"
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
        "date": 1744300345281,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 155.25769478056202,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 165.78815115093468,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 8005.366756108016,
            "unit": "milliseconds"
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
        "date": 1744352904438,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 152.58153282327498,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 168.62951863987436,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 7640.126795665211,
            "unit": "milliseconds"
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
        "date": 1744533782437,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 151.82690272535433,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 174.12894424106568,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 8022.183268390763,
            "unit": "milliseconds"
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
        "date": 1744628428876,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 151.61521819071535,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 175.17074850250137,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6860.569481476695,
            "unit": "milliseconds"
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
        "date": 1745136377801,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 151.85325789281765,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 166.648544814086,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6867.745554737206,
            "unit": "milliseconds"
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
        "date": 1745741025684,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 155.70467461847966,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 165.36271647175334,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 7823.288035583087,
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
          "id": "4ddfcd4b776650ae5172663c04db2a0fb791cbd6",
          "message": "Fix logs using up all the space of /tmp folder (#1723)",
          "timestamp": "2025-05-03T08:50:17Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4ddfcd4b776650ae5172663c04db2a0fb791cbd6"
        },
        "date": 1746346457912,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 157.484783287365,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 166.87221784282931,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 7763.213719014086,
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
          "id": "4ddfcd4b776650ae5172663c04db2a0fb791cbd6",
          "message": "Fix logs using up all the space of /tmp folder (#1723)",
          "timestamp": "2025-05-03T08:50:17Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4ddfcd4b776650ae5172663c04db2a0fb791cbd6"
        },
        "date": 1746951322840,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 163.911592773332,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 161.114123114414,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 17649.15693313919,
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
          "id": "4ddfcd4b776650ae5172663c04db2a0fb791cbd6",
          "message": "Fix logs using up all the space of /tmp folder (#1723)",
          "timestamp": "2025-05-03T08:50:17Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4ddfcd4b776650ae5172663c04db2a0fb791cbd6"
        },
        "date": 1747555913923,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 151.607298906114,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 173.154229553696,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 7503.870633714309,
            "unit": "milliseconds"
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
        "date": 1748161045744,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 153.75636648874368,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 171.8954910532283,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 7353.818629827899,
            "unit": "milliseconds"
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
        "date": 1748766457493,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 150.9647070950863,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 162.99193031241865,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 7360.315872294741,
            "unit": "milliseconds"
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
        "date": 1749370711863,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 155.95899856235698,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 167.55029336909368,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 8082.091221851692,
            "unit": "milliseconds"
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
        "date": 1749975309777,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 152.750085590238,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 168.05755060062302,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 7997.411555278497,
            "unit": "milliseconds"
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
        "date": 1750580916454,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 153.98355153219634,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 167.33765874414365,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 7888.024650268179,
            "unit": "milliseconds"
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
        "date": 1751189622622,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 303.18027219594336,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 177.67875257294733,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 8606.488075164054,
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
          "id": "9fd527bf3c9d1c94ddb1f97083248208392e9fdb",
          "message": "fix rhel package installer in nightly pipeline (#1853)",
          "timestamp": "2025-07-04T12:07:45Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/9fd527bf3c9d1c94ddb1f97083248208392e9fdb"
        },
        "date": 1751790191854,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 158.34719925378334,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 165.58355540608366,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 7847.738294029346,
            "unit": "milliseconds"
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
        "date": 1752395091719,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 158.60913750711566,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 166.597146620882,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 7671.868404828941,
            "unit": "milliseconds"
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
        "date": 1753000140364,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 166.660970599818,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 160.56732253663998,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 7184.52108154239,
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
          "id": "7776998e3f031e791148482db29f8c40beb53255",
          "message": "Add New stage to the Nightly pipeline (#1889)",
          "timestamp": "2025-07-24T07:31:00Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/7776998e3f031e791148482db29f8c40beb53255"
        },
        "date": 1753605132141,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 155.341961322253,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 162.73388105601333,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 7385.644559433046,
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
          "id": "1a7d91ce786c8012c8afe26308e2c4d05fefd6aa",
          "message": "fix e2e tests failure  (#1913)\n\nCo-authored-by: Copilot <175728472+Copilot@users.noreply.github.com>",
          "timestamp": "2025-08-05T09:59:31Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/1a7d91ce786c8012c8afe26308e2c4d05fefd6aa"
        },
        "date": 1755418838553,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 156.97786864111436,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 163.727518600093,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 7275.6962358720275,
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
          "id": "1a7d91ce786c8012c8afe26308e2c4d05fefd6aa",
          "message": "fix e2e tests failure  (#1913)\n\nCo-authored-by: Copilot <175728472+Copilot@users.noreply.github.com>",
          "timestamp": "2025-08-05T09:59:31Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/1a7d91ce786c8012c8afe26308e2c4d05fefd6aa"
        },
        "date": 1756023650168,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 157.65157244014802,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 163.27331838027234,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 8058.2125499861795,
            "unit": "milliseconds"
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
        "date": 1756628367827,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 154.917312137885,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 161.20235743726266,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 7351.669281021889,
            "unit": "milliseconds"
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
        "date": 1758442122571,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 159.345714563536,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 166.79051340955334,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 7295.3541820381,
            "unit": "milliseconds"
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
        "date": 1759047433477,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 155.9829594237253,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 163.435298086034,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 7445.302344069881,
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
          "id": "43314da664fe649d926fa148b6253ae28dff8d3f",
          "message": "Add FIO tests to check the data integrity (#1893)",
          "timestamp": "2025-09-29T10:20:04Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/43314da664fe649d926fa148b6253ae28dff8d3f"
        },
        "date": 1759652556847,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 156.19998123930466,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 186.811974389516,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 7317.891029266638,
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
          "id": "389136cf285c96aae19b8e61c3c5bb0cee98bb45",
          "message": "Fix issues while truncating the file (#2003)\n\nCo-authored-by: Vikas Bhansali <64532198+vibhansa-msft@users.noreply.github.com>\nCo-authored-by: vibhansa <vibhansa@microsoft.com>",
          "timestamp": "2025-10-10T08:30:20Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/389136cf285c96aae19b8e61c3c5bb0cee98bb45"
        },
        "date": 1760107814694,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 156.355565821983,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 198.69497582382166,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 7423.561016023013,
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
          "id": "389136cf285c96aae19b8e61c3c5bb0cee98bb45",
          "message": "Fix issues while truncating the file (#2003)\n\nCo-authored-by: Vikas Bhansali <64532198+vibhansa-msft@users.noreply.github.com>\nCo-authored-by: vibhansa <vibhansa@microsoft.com>",
          "timestamp": "2025-10-10T08:30:20Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/389136cf285c96aae19b8e61c3c5bb0cee98bb45"
        },
        "date": 1760262689999,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 156.180707200265,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 198.88142221267367,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 7425.652555900746,
            "unit": "milliseconds"
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
        "date": 1760866894272,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 158.00580740547198,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 201.05953088028267,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 7031.688439958062,
            "unit": "milliseconds"
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
        "date": 1761472236289,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 157.84939703238066,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 190.463577969394,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 7142.453715105356,
            "unit": "milliseconds"
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
        "date": 1762076150564,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 155.86803781940466,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 189.18266756127866,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 7086.156190376466,
            "unit": "milliseconds"
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
        "date": 1762681587841,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 158.27708995649,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 194.76628267829565,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 7327.615613526995,
            "unit": "milliseconds"
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
        "date": 1763287097799,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 158.63640534096268,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 182.44913326089195,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6915.154832326575,
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
          "id": "f17c24583d29f72cf67eb652d27f482f87ecdc9f",
          "message": "Build support for arm32 (#2068)",
          "timestamp": "2025-11-21T09:53:19Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/f17c24583d29f72cf67eb652d27f482f87ecdc9f"
        },
        "date": 1763891571676,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 157.60107295097632,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 182.030386737648,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 7207.16292562489,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}