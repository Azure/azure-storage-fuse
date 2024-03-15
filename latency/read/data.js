window.BENCHMARK_DATA = {
  "lastUpdate": 1710494139801,
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
        "date": 1710434160601,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 1.8770822192866667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 72.95500704929933,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.40159898902566665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.6819019531466667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1.909903444749,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 71.41478988569033,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 2.3104025634350003,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 4.2121870510949995,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_four_threads",
            "value": 72.506766126339,
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
          "id": "b8e905041b73f9a1a1b6e9d8c20adccde8061ff5",
          "message": "Correcting condition",
          "timestamp": "2024-03-14T22:28:41+05:30",
          "tree_id": "761b9036e9e7ecefa4bff1acfa88658816f1f82b",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/b8e905041b73f9a1a1b6e9d8c20adccde8061ff5"
        },
        "date": 1710437906084,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 1.8760104535266666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 76.00654636732833,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.40077807351700007,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.856094331578,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1.9371153429066668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 72.80917143922834,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 2.3311029756493333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 4.013385819402333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_four_threads",
            "value": 75.66196319188799,
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
          "id": "3aa00a115fcb6908e06e8076bf1008676d15a5ab",
          "message": "Correct the condition",
          "timestamp": "2024-03-15T11:11:13+05:30",
          "tree_id": "2ec41c263addeb219ba6852d3915d312f1a1dc1e",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/3aa00a115fcb6908e06e8076bf1008676d15a5ab"
        },
        "date": 1710483404260,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 1.8825867909783334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 70.12149857096365,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.324934166149,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.6870573233380001,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1.9207579680506666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 70.48468802682466,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 2.3604285356056667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 3.2403372495056666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_four_threads",
            "value": 71.10623437259066,
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
          "id": "d5a42bfdbf8578b4a6954fcccc8d60b56280ad49",
          "message": "Seperate out list test",
          "timestamp": "2024-03-15T11:53:06+05:30",
          "tree_id": "78f0ba1b4fbd48d7fb562d98712bfd6c14f19b2e",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/d5a42bfdbf8578b4a6954fcccc8d60b56280ad49"
        },
        "date": 1710485849539,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 1.8461946527026665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 72.28100296875199,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.3629602647956667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.8241723046396667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1.953647741694,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 71.729258949711,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 2.3326815888823336,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 3.5870660621289994,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_four_threads",
            "value": 72.99539729577499,
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
          "id": "d5a42bfdbf8578b4a6954fcccc8d60b56280ad49",
          "message": "Seperate out list test",
          "timestamp": "2024-03-15T11:53:06+05:30",
          "tree_id": "78f0ba1b4fbd48d7fb562d98712bfd6c14f19b2e",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/d5a42bfdbf8578b4a6954fcccc8d60b56280ad49"
        },
        "date": 1710494139468,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 1.860063778212,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 75.81718760112834,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.37478294037666665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.7634068452939999,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1.933201032926,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 70.48030361988499,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 2.314817806249667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 4.139167132748667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_four_threads",
            "value": 72.63230293171101,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}