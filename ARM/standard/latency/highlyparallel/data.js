window.BENCHMARK_DATA = {
  "lastUpdate": 1760116096230,
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
        "date": 1741577866004,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 106828.02197551087,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 65260.104923546045,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 110787.19100495208,
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
        "date": 1742387180780,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 194.62624718550032,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 167.242904152828,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 9673.013667370002,
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
        "date": 1744279498005,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 183.133856358588,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 175.82046605759834,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 8973.508505566928,
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
        "date": 1744358525389,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 205.31134294492335,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 164.368537939854,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 9651.650472211026,
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
        "date": 1744539898044,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 187.44019755091267,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 165.17023120285864,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 8623.404730022618,
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
        "date": 1744634385149,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 185.006584453982,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 167.67418116737568,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 8780.608049475153,
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
        "date": 1745142325206,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 192.01332692406467,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 164.81614605258798,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 8750.48964657555,
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
        "date": 1745746929039,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 185.79382767864968,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 173.7247414618803,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 9098.042149167266,
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
        "date": 1746352325153,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 194.33385690004465,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 169.62808749616502,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 9419.99851899901,
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
        "date": 1746956984922,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 190.308711528122,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 170.723417721961,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 8801.348865927685,
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
        "date": 1747561731621,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 197.7911236931233,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 176.404837977036,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 8817.145406374764,
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
        "date": 1748166932636,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 182.03208368896733,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 166.553772591184,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 8317.650408571892,
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
        "date": 1748772257120,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 191.47190692939435,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 170.52031185251602,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 9322.253697726059,
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
        "date": 1749376402224,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 198.88686361024335,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 166.54874954486834,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 9030.12786656265,
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
        "date": 1749981084110,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 194.30334233279567,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 166.04045849693534,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 9254.578261855813,
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
        "date": 1750586657968,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 193.6028733755983,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 171.749671455252,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 9378.466029518784,
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
        "date": 1751195702504,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 199.002231711314,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 161.68145590134768,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 9128.490712396942,
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
        "date": 1751796092164,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 195.56284773476798,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 169.22132175091932,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 9272.372779888232,
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
        "date": 1752401302513,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 199.87763245169467,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 173.257355641523,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 10011.443015660145,
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
        "date": 1753006058982,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 204.55984158957565,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 171.575726594722,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 9599.220624587564,
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
        "date": 1758448052589,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 206.921978695809,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 170.298391787666,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 9341.289080859859,
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
        "date": 1759053274730,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 202.69791929391434,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 162.91438148071333,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 9871.25449280611,
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
        "date": 1759658414391,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 196.90924455527102,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 209.86282422503032,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 8978.72988612382,
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
        "date": 1760116095939,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 199.51979531820032,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 195.49843758496135,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 8616.69555445756,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}