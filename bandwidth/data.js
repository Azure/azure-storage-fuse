window.BENCHMARK_DATA = {
  "lastUpdate": 1709314128332,
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
          "id": "48f3a126598ed59103a1e65a6363115cb01232e1",
          "message": "Correcting test names",
          "timestamp": "2024-03-01T21:59:50+05:30",
          "tree_id": "b85e5a25effbb090c48b554decb0b307495f580c",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/48f3a126598ed59103a1e65a6363115cb01232e1"
        },
        "date": 1709314126441,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "Seq_Write_40thread",
            "value": 0,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_four_threads",
            "value": 26.128580729166668,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 7.135416666666667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 525.3538411458334,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.6852213541666665,
            "unit": "MiB/s"
          },
          {
            "name": "Seq_Write_128_thread",
            "value": 0,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 3196.3935546875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 1441.5651041666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 427.8902994791667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 1622.638671875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read",
            "value": 419.2112630208333,
            "unit": "MiB/s"
          },
          {
            "name": "Seq_Write_100_thread",
            "value": 20284.454427083332,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write",
            "value": 1225.1474609375,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}