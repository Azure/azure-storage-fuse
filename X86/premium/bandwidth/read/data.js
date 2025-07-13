window.BENCHMARK_DATA = {
  "lastUpdate": 1752381620781,
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
          "id": "e531bd70068a4d7ecb9c2d1096f6cf78f31eadb7",
          "message": "Updated container name",
          "timestamp": "2025-03-07T03:39:29-08:00",
          "tree_id": "d61a69967fd61b62788c82e601611c72fb11db2a",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e531bd70068a4d7ecb9c2d1096f6cf78f31eadb7"
        },
        "date": 1741348714260,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2201.9817708333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.3564453125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2351.685546875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1263.37109375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2209.3343098958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.4140625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4555.583658854167,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3899.298828125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.3515625,
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
          "id": "db543abdaf167da96dc1aab0033b0b26c065bf7c",
          "message": "Added step to cleanup block-cache temp path on start",
          "timestamp": "2025-03-07T04:43:37-08:00",
          "tree_id": "8efca96f31bbb941ccd6e7c17a880599f40282f3",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/db543abdaf167da96dc1aab0033b0b26c065bf7c"
        },
        "date": 1741352831255,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2278.1263020833335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.3512369791666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2340.1520182291665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1288.6204427083333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2397.3473307291665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.2213541666666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4693.631184895833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3682.310546875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 12.778971354166666,
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
          "id": "03e72e47d37985e5c28051c0ff17bdc0c7315e74",
          "message": "Correcting code for cache cleanup",
          "timestamp": "2025-03-07T23:48:22-08:00",
          "tree_id": "5b69276c81c0c728ae2dd3889b9743194fdcc990",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/03e72e47d37985e5c28051c0ff17bdc0c7315e74"
        },
        "date": 1741421380261,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2473.2418619791665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.3274739583333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2816.3525390625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1241.5634765625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2304.8951822916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.1832682291666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4399.29296875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4161.346028645833,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 12.428059895833334,
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
        "date": 1741627309180,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2376.6787109375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.248046875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2837.5270182291665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1636.919921875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2348.8961588541665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.3264973958333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4862.768880208333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4105.681315104167,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.103515625,
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
          "id": "3b8c1479c707311289f5ee84bf3770b0956497d9",
          "message": "Merge branch 'vibhansa/armperftest' of https://github.com/Azure/azure-storage-fuse into vibhansa/armperftest",
          "timestamp": "2025-03-10T19:45:26-07:00",
          "tree_id": "36cf3d2ea59d4e24ae14350add991d98e2be0d9c",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/3b8c1479c707311289f5ee84bf3770b0956497d9"
        },
        "date": 1741662427912,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2464.4951171875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.4124348958333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2643.7584635416665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1469.0784505208333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2557.6920572916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.529296875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4764.473307291667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4528.123697916667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.385416666666666,
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
          "id": "4743391f1eac34ad882c8766eb0ee100a2850101",
          "message": "Merge branch 'main' into vibhansa/armperftest",
          "timestamp": "2025-03-13T15:25:11+05:30",
          "tree_id": "d38731698647b2856b859fa97b173461cbae6803",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4743391f1eac34ad882c8766eb0ee100a2850101"
        },
        "date": 1741860975841,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2492.3134765625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.1630859375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2745.9108072916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1181.8157552083333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2494.9915364583335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.2242838541666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4703.141276041667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4310.015950520833,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 12.873697916666666,
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
          "id": "4987ab98e4f8a27a7df0de21978e5ab610135a4d",
          "message": "Remove disk caching from the bench pipeline",
          "timestamp": "2025-03-19T06:25:20Z",
          "tree_id": "0e4f061068b5dcb8afee11fa25a6dcfe27b0d5ef",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4987ab98e4f8a27a7df0de21978e5ab610135a4d"
        },
        "date": 1742366825486,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2089.6324869791665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.2919921875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2133.3811848958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1325.3971354166667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2661.7412109375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.2337239583333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4541.078125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3961.6520182291665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.101236979166666,
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
        "date": 1742370164076,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2173.2298177083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.2307942708333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2779.75390625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1265.2825520833333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2677.6686197916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.2747395833333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4760.331380208333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3960.224609375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.534830729166666,
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
        "date": 1744262202328,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2426.7600911458335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.6409505208333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2613.2679036458335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1221.3505859375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2409.619140625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.2952473958333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4867.427408854167,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4132.994140625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.185872395833334,
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
          "id": "a255ef1080a4bee70d172d6b9d86109bc75a69ae",
          "message": "Updating configs",
          "timestamp": "2025-04-10T02:02:50-07:00",
          "tree_id": "eab286874e9c1ba63c0fe44c0be17d45750a7853",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/a255ef1080a4bee70d172d6b9d86109bc75a69ae"
        },
        "date": 1744277267298,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2418.310546875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 2.9934895833333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2939.2789713541665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 3211.5621744791665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2564.7613932291665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.2044270833333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 7601.876627604167,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 8416.212565104166,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 12.4091796875,
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
        "date": 1744286674003,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2275.259765625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.0859375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2169.2164713541665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1466.4586588541667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2484.9029947916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.4111328125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4943.884765625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4510.703776041667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 12.1689453125,
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
          "id": "0cd7596da267f6a7f6ad7cd9126fc9ae1305d3bd",
          "message": "Adding arm based benchmark tests (#1654)\n\nCo-authored-by: Srinivas Yeleti <syeleti@microsoft.com>",
          "timestamp": "2025-04-10T21:13:25+05:30",
          "tree_id": "476165a49a1855d8214edc6bb7530574e73094a2",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/0cd7596da267f6a7f6ad7cd9126fc9ae1305d3bd"
        },
        "date": 1744301210057,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2240.6165364583335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.01171875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2735.1197916666665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1373.5595703125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2598.4046223958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.0947265625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 5011.978190104167,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4422.8408203125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 12.723307291666666,
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
        "date": 1744339743401,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2125.478515625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.2545572916666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2890.564453125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1502.5244140625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2580.3551432291665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.3352864583333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4765.035481770833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4335.505533854167,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 12.967447916666666,
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
        "date": 1744519970048,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2268.9156901041665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.2731119791666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2529.7486979166665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1352.5738932291667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2692.9235026041665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.2561848958333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 5069.394856770833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4463.395182291667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.094075520833334,
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
        "date": 1744614652864,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2374.9287109375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.3958333333333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2536.3844401041665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1427.6793619791667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2283.3177083333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.3356119791666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4888.258463541667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4127.458333333333,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 12.339518229166666,
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
        "date": 1745123099939,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2399.8004557291665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.3277994791666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2862.0680338541665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1431.2158203125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2679.9514973958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.341796875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4843.560221354167,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4348.9541015625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 12.783854166666666,
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
        "date": 1745308264872,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 1102.9729817708333,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.1575520833333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2436.8323567708335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1328.1162109375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1465.5166015625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.0970052083333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 3935.5491536458335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 2942.9563802083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 12.277994791666666,
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
        "date": 1745727948288,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2289.6119791666665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.5563151041666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2518.8014322916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1267.5823567708333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2526.5152994791665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.4423828125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4188.877278645833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4194.7822265625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.091796875,
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
        "date": 1746332845098,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2430.9713541666665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.060546875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2692.9654947916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1378.71484375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2352.9378255208335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.4609375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4810.178059895833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4175.524739583333,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.444661458333334,
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
        "date": 1746937638489,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2221.8313802083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.3414713541666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 3015.8795572916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1221.38671875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2318.5807291666665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.6875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 2306.517578125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1960.5628255208333,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 9.5537109375,
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
        "date": 1747542561502,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2588.4661458333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.3098958333333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2589.8463541666665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1239.3610026041667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2473.5641276041665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.296875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4718.7685546875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4433.205403645833,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.616536458333334,
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
        "date": 1748147394784,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2400.8167317708335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.1917317708333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2666.5465494791665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1220.6640625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2405.7998046875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.5947265625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 5089.834635416667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4251.007161458333,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 12.942057291666666,
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
        "date": 1748752673241,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2442.7132161458335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.9189453125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2261.4654947916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1199.365234375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2522.9208984375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.4371744791666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4518.402669270833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4305.520833333333,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.678385416666666,
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
        "date": 1749357192765,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2356.7184244791665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.5716145833333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2430.1380208333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1383.333984375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2701.83984375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.837890625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 5152.900716145833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4485.607096354167,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.925455729166666,
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
        "date": 1749961922230,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2679.8548177083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.408203125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2566.1640625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1396.43359375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2614.990234375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.3255208333333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4861.651692708333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4311.774088541667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 14.292643229166666,
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
        "date": 1750566781514,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2416.2799479166665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.3736979166666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2824.8860677083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1637.2643229166667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2593.1188151041665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.5139973958333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4809.7041015625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4433.3193359375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 14.5341796875,
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
        "date": 1751171777851,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2683.5989583333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.2464192708333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2911.572265625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1207.3981119791667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2737.8414713541665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.4671223958333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 5109.468424479167,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4465.629557291667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.354817708333334,
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
        "date": 1751776574033,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2514.7574869791665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.4319661458333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2604.2490234375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1241.5690104166667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2690.0348307291665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.8020833333333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4890.6025390625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4534.4775390625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.500325520833334,
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
        "date": 1752381619364,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2723.6809895833335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.3997395833333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2482.3776041666665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1181.0911458333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2724.189453125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.3961588541666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 5033.14453125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4539.4775390625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.971028645833334,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}