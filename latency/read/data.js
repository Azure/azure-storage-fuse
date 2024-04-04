window.BENCHMARK_DATA = {
  "lastUpdate": 1712246931727,
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
        "date": 1712228210476,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09237264668933333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 69.102910703463,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.092863626107,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.13744655439099998,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09200917992699999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 72.49075391474766,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17470297946866667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9887939912203335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 75.59788254316534,
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
          "id": "af6c4b7f5027b190ed6fd22b9411c121cde95161",
          "message": "Sync with main",
          "timestamp": "2024-04-04T21:19:24+05:30",
          "tree_id": "8d44ebd434f1348fa4eccb4120623d585257ea2e",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/af6c4b7f5027b190ed6fd22b9411c121cde95161"
        },
        "date": 1712246931417,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10414171725733333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 72.529268732636,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.099086034998,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.21138560931633332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09646497864033332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 73.81312104925601,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18145079937999997,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0453324132596666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 75.48258914304967,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}