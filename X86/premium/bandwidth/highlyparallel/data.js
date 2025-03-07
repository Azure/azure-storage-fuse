window.BENCHMARK_DATA = {
  "lastUpdate": 1741353607670,
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
        "date": 1741349492112,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 24131.076497395832,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 25056.130859375,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 4319.4111328125,
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
        "date": 1741353607404,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 24226.3388671875,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 24947.865559895832,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 4149.950846354167,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}