window.BENCHMARK_DATA = {
  "lastUpdate": 1712680823020,
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
          "id": "1a4e554337ce8799974951b862bc67522031adf1",
          "message": "Correcting bs in large write case",
          "timestamp": "2024-04-04T15:58:04+05:30",
          "tree_id": "4a43bfe9042ae83dab8725a2ba1ea42ed150b950",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/1a4e554337ce8799974951b862bc67522031adf1"
        },
        "date": 1712228208615,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2482.5139973958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.6279296875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2336.6272786458335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1649.4833984375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2708.4593098958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.4537760416666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4761.986979166667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3977.8662109375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.228515625,
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
          "id": "af6c4b7f5027b190ed6fd22b9411c121cde95161",
          "message": "Sync with main",
          "timestamp": "2024-04-04T21:19:24+05:30",
          "tree_id": "8d44ebd434f1348fa4eccb4120623d585257ea2e",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/af6c4b7f5027b190ed6fd22b9411c121cde95161"
        },
        "date": 1712246929661,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2186.5791015625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.4495442708333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2211.4801432291665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1105.3404947916667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2586.2327473958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.38671875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4783.632161458333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3722.6256510416665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.223958333333334,
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
          "id": "efc39a9a7a9ade6bef2ade06f5134a61ca3708c8",
          "message": "Merge remote-tracking branch 'origin/main' into vibhansa/perftestrunner",
          "timestamp": "2024-04-09T21:50:08+05:30",
          "tree_id": "919ec536002591c79c706b99acb15eccd3353c73",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/efc39a9a7a9ade6bef2ade06f5134a61ca3708c8"
        },
        "date": 1712680821983,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2345.607421875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.4596354166666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2386.4059244791665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1628.9202473958333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1809.7392578125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.3313802083333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4668.119140625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3796.8642578125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.612955729166666,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}