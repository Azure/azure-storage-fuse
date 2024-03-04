window.BENCHMARK_DATA = {
  "lastUpdate": 1709535174825,
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
          "id": "0ef8cba41c61a633c2e444937bad684014be6df6",
          "message": "Correcting file",
          "timestamp": "2024-03-04T11:18:50+05:30",
          "tree_id": "cceff9e3635fd28153793f3d1617ccc4d2b67116",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/0ef8cba41c61a633c2e444937bad684014be6df6"
        },
        "date": 1709535171994,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "Seq_Write_128_thread",
            "value": 0,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_four_threads",
            "value": 26.1826171875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 6.309244791666667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1100.962890625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 6.6220703125,
            "unit": "MiB/s"
          },
          {
            "name": "Seq_Write_128_thread",
            "value": 0,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 2652.0550130208335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 1448.9332682291667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 420.5260416666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2167.2386067708335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read",
            "value": 416.3681640625,
            "unit": "MiB/s"
          },
          {
            "name": "Seq_Write_100_thread",
            "value": 20854.900716145832,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write",
            "value": 1174.1604817708333,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}