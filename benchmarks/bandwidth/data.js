window.BENCHMARK_DATA = {
  "lastUpdate": 1709314153406,
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
          "id": "fb2d21c3e64a5c241063a6a9a8338f76b9d22220",
          "message": "Correcting names",
          "timestamp": "2024-03-01T22:02:18+05:30",
          "tree_id": "b6ba8567870b3c59c21be7cdca6c9b942d27477c",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/fb2d21c3e64a5c241063a6a9a8338f76b9d22220"
        },
        "date": 1709314150585,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "Seq_Write_128_thread",
            "value": 0,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_four_threads",
            "value": 27.8583984375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 6.8232421875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 699.8430989583334,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 4.780924479166667,
            "unit": "MiB/s"
          },
          {
            "name": "Seq_Write_128_thread",
            "value": 0,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 3215.85546875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 1452.7083333333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 427.2333984375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 1594.357421875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read",
            "value": 419.7138671875,
            "unit": "MiB/s"
          },
          {
            "name": "Seq_Write_100_thread",
            "value": 20200.88671875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write",
            "value": 1165.1604817708333,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}