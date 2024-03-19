window.BENCHMARK_DATA = {
  "lastUpdate": 1710846484647,
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
          "id": "c272d8b5b8f90fe9671356efd3e7587be836a870",
          "message": "Correct ioengine",
          "timestamp": "2024-03-14T21:30:19+05:30",
          "tree_id": "97809478e0f900cc621a2be24358a30eb6eef4f7",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/c272d8b5b8f90fe9671356efd3e7587be836a870"
        },
        "date": 1710434158439,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 519.2972005208334,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 13.709635416666666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2233.2470703125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1372.3759765625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 523.3974609375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 14.0322265625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 1584.2171223958333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 3708.9713541666665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_four_threads",
            "value": 54.895182291666664,
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
          "id": "b8e905041b73f9a1a1b6e9d8c20adccde8061ff5",
          "message": "Correcting condition",
          "timestamp": "2024-03-14T22:28:41+05:30",
          "tree_id": "761b9036e9e7ecefa4bff1acfa88658816f1f82b",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/b8e905041b73f9a1a1b6e9d8c20adccde8061ff5"
        },
        "date": 1710437902257,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 518.7858072916666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 13.194986979166666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2192.4264322916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1139.1061197916667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 516.1637369791666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 13.737955729166666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 1584.6832682291667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 3885.7080078125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_four_threads",
            "value": 52.8095703125,
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
          "id": "3aa00a115fcb6908e06e8076bf1008676d15a5ab",
          "message": "Correct the condition",
          "timestamp": "2024-03-15T11:11:13+05:30",
          "tree_id": "2ec41c263addeb219ba6852d3915d312f1a1dc1e",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/3aa00a115fcb6908e06e8076bf1008676d15a5ab"
        },
        "date": 1710483402150,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 516.96484375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 14.252604166666666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2730.0530598958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1381.5309244791667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 520.4856770833334,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 14.185872395833334,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 1564.3902994791667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 4791.3017578125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_four_threads",
            "value": 55.956380208333336,
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
          "id": "d5a42bfdbf8578b4a6954fcccc8d60b56280ad49",
          "message": "Seperate out list test",
          "timestamp": "2024-03-15T11:53:06+05:30",
          "tree_id": "78f0ba1b4fbd48d7fb562d98712bfd6c14f19b2e",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/d5a42bfdbf8578b4a6954fcccc8d60b56280ad49"
        },
        "date": 1710485847679,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 526.962890625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 13.833984375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2404.8479817708335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1136.4505208333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 511.8115234375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 13.947591145833334,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 1576.8414713541667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 4363.563802083333,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_four_threads",
            "value": 54.4541015625,
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
          "id": "d5a42bfdbf8578b4a6954fcccc8d60b56280ad49",
          "message": "Seperate out list test",
          "timestamp": "2024-03-15T11:53:06+05:30",
          "tree_id": "78f0ba1b4fbd48d7fb562d98712bfd6c14f19b2e",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/d5a42bfdbf8578b4a6954fcccc8d60b56280ad49"
        },
        "date": 1710494137397,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 522.9342447916666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 13.183268229166666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2330.2887369791665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1244.19140625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 517.0904947916666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 14.187174479166666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 1589.4654947916667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 3770.2903645833335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_four_threads",
            "value": 54.786458333333336,
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
          "id": "9655fa496a1d7e09bd207e849d699f9475bf1010",
          "message": "Make write the last test case",
          "timestamp": "2024-03-15T15:28:45+05:30",
          "tree_id": "ca4ee27f4eac8d5db3140c08802b3d5984e8d09f",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/9655fa496a1d7e09bd207e849d699f9475bf1010"
        },
        "date": 1710498784251,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 512.0234375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 13.298177083333334,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2046.8203125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1382.4645182291667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 515.3483072916666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 13.521484375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 1569.7529296875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 4165.707356770833,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_four_threads",
            "value": 51.859700520833336,
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
          "id": "3f6a9fe604bef52d7cb679a7d108023e4336708c",
          "message": "Silent the clogs in case of creation",
          "timestamp": "2024-03-18T12:04:44+05:30",
          "tree_id": "9c2ea968b5fa3bff2feea572aa84a9cba678b72c",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/3f6a9fe604bef52d7cb679a7d108023e4336708c"
        },
        "date": 1710745709443,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 528.6803385416666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 13.654947916666666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2253.8577473958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1192.6708984375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 523.5865885416666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 13.816731770833334,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 1581.0094401041667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 4204.528971354167,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_four_threads",
            "value": 54.192057291666664,
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
          "id": "3bc15cf143c5c8ccfbe50dce3f4ee190c9a6fe02",
          "message": "Reset open files setting",
          "timestamp": "2024-03-19T15:31:59+05:30",
          "tree_id": "bcd3da2f495e1f5e3719d8bae7fb32b00f9ec479",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/3bc15cf143c5c8ccfbe50dce3f4ee190c9a6fe02"
        },
        "date": 1710846484338,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 521.7760416666666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 13.378255208333334,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2130.9931640625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1170.1494140625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 526.9098307291666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 13.733723958333334,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 1586.4596354166667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 4269.904947916667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_four_threads",
            "value": 56.480794270833336,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}