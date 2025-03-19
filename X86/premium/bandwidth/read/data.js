window.BENCHMARK_DATA = {
  "lastUpdate": 1742370165340,
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
      }
    ]
  }
}